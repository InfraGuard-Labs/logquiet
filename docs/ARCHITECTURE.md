# Architecture

## Language choice: Go

Go was chosen over Rust (the other candidate named in the project brief)
for this specific tool, for reasons specific to this tool - not as a
general "Go is better" claim:

- **Standalone, dependency-free cross-compilation.** `GOOS=linux
  GOARCH=arm64 go build` produces a working static-ish binary for another
  platform from any development machine, no target toolchain or C cross-
  compiler needed. This mattered concretely during development: the
  binaries for all five release targets (Linux amd64/arm64, macOS
  Intel/Apple Silicon, Windows amd64) are built from one Windows
  development machine with nothing beyond the stock Go SDK. Rust's
  cross-compilation story (target toolchains, sometimes a C linker per
  target) is comparatively heavier for a project that wants a trivial
  release process.
- **Fast enough for the job.** Streaming line-oriented text processing at
  the sustained rates real `kubectl logs -f`/`tail -F` streams actually
  produce (typically well under a few thousand lines/sec, even for noisy
  services) is well within reach of Go's performance envelope; see
  [BENCHMARKS.md](BENCHMARKS.md) for measured throughput, which has
  comfortable headroom over realistic live-tailing rates even though it is
  well behind a hand-tuned Rust implementation would likely achieve.
- **Low operational memory**, controllable directly: Go's GC and the
  explicit bounded data structures used here (`internal/pattern.Store`'s
  LRU eviction, fixed-size ring buffers for rolling rates, a hard cap on
  multiline block size and line length) give predictable memory behavior
  without needing arena allocators or manual memory management.
- **Standard library sufficiency.** The entire implementation uses only
  the Go standard library - no third-party dependency at all (`go.mod` has
  no `require` lines beyond the module declaration itself). This was a
  deliberate simplicity and supply-chain choice (see
  [SECURITY.md](../SECURITY.md)): there is nothing to audit for
  unexpected network behavior beyond the code in this repository.
- **CLI ecosystem is adequate for a single-purpose tool.** LogQuiet's flag
  surface is small enough that the standard `flag` package, with a
  hand-written `-help` banner, is sufficient; pulling in a larger CLI
  framework would add a dependency for no real benefit here.

Rust would likely win on raw throughput ceiling and per-line memory if
that ceiling were the binding constraint for this product; it is not - a
terminal log-tailing tool is bound by "comfortably faster than any human
or real infrastructure log stream," not by matching `grep`'s throughput on
synthetic benchmarks. Go's development velocity and packaging simplicity
were judged more valuable for this project's actual constraints.

## Package layout

```
cmd/logquiet/         entry point: flag parsing, I/O wiring, signal handling
internal/severity/     level detection (bracketed and bare forms)
internal/logline/      per-line prefix (timestamp/level) stripping
internal/multiline/     stack-trace/traceback continuation heuristics
internal/normalize/     structural template normalization
internal/fingerprint/   template -> stable 64-bit ID
internal/pattern/       per-fingerprint state, LRU-bounded store, anomaly detection
internal/render/        terminal/plain/JSON output, repeat accumulation and flush
internal/pipeline/      wires the above into the per-line decision
internal/stats/         session counters and the --impact-report schema
internal/config/        flag parsing and defaults
internal/reader/        bounded-length line splitting for bufio.Scanner
tools/fixturegen/       generates fixtures/synthetic/* and benchmarks/data/* (not shipped)
benchmarks/correctness/ runs the pipeline against every fixture and checks retention
```

Each `internal/*` package has a single, narrow responsibility and no
package depends on `cmd/logquiet` or on `internal/pipeline` except
`pipeline` itself - `pipeline` is the only package that knows how all the
pieces compose. This was a deliberate choice so each stage (normalization,
multiline grouping, anomaly detection, rendering) could be unit-tested and
benchmarked in complete isolation, which is how the interleaved-pattern
suppression bug and the normalization performance bottleneck (see
[TECHNICAL_METHOD.md](TECHNICAL_METHOD.md) and
[BENCHMARKS.md](BENCHMARKS.md)) were actually found during development -
by benchmarking and correctness-testing packages independently, not just
eyeballing end-to-end output.

## Data flow for one input line

```
main.go: bufio.Scanner (bounded line length)
  -> pipeline.ProcessLine(raw, now)
       -> logline.Extract(raw)              severity + prefix-stripped content
       -> multiline.Assembler.Feed(...)     may return a completed Block, or none yet
       -> [if a Block completed] handleBlock:
            -> normalize.Template(content) per line in the block, joined
            -> fingerprint.Of(severity, template)
            -> pattern.Store.GetOrCreate(...)   -> State, isNew
            -> State.Record(now)                -> *Spike or nil
            -> decide: anomaly | novel (Emit) | repeat (Accumulate)
       -> renderer.Tick(now)                 flush any due repeat counters
main.go: on EOF/SIGINT/SIGTERM -> pipeline.Finish(now) -> flush trailing block + all pending counters
```

## Concurrency model

The pipeline is single-threaded by design: one goroutine reads stdin/file
and drives the entire pipeline synchronously. A second goroutine only
exists to catch `os.Interrupt`/`SIGTERM` and trigger the same finalization
path the main loop would run at EOF. This is a deliberate simplicity choice
appropriate to the workload (a single ordered stream in, a single ordered
stream out) - there is no benefit to parallelizing stages that must run in
strict input order and share mutable state (the pattern store, the
renderer's pending counters).

## Terminal rendering modes

- **TTY** (default when stdout is a terminal): ANSI color by severity.
- **Plain** (`--plain`, or automatically when stdout is not a terminal):
  no color, no cursor control.
- **No-color** (`--no-color`): like TTY but without ANSI color codes.
- **JSON** (`--json`): newline-delimited JSON. The three stable `type`
  values are `event`, `repeat_summary`, and `anomaly` - a `repeat_summary`
  is emitted both for periodic flushes during a long-running stream and
  for the final flush of a pattern's count at EOF/shutdown; there is no
  separate event type for the latter. See the README for the field
  reference.

TTY detection is done with the standard-library-only check
`(os.Stdout.Stat().Mode() & os.ModeCharDevice) != 0`, requiring no
third-party terminal library.

## Exit and error handling

- **EOF** on stdin/file: the normal, expected end of a bounded input;
  `pipeline.Finish` flushes trailing state and the process exits 0.
- **SIGINT/SIGTERM**: caught, triggers the same finalization path, then
  `os.Exit(0)`. The main read loop is blocked in a syscall at the moment a
  signal arrives (not concurrently mutating shared state), so this is safe
  in practice despite not using a mutex-free design throughout; a mutex
  does guard the shared counters/pipeline state between the two goroutines
  for correctness under the Go race detector (`go test -race` is part of
  CI).
- **Closed downstream pipe** (e.g. `logquiet | head`): writes are wrapped
  in a small `safeWriter` that detects `EPIPE`/`ECONNRESET` and becomes a
  no-op afterward, and the main loop stops reading input once output is
  detected as broken - rather than the default Go behavior of panicking on
  a broken-pipe write.
- **Malformed/oversized input**: `internal/reader.BoundedLines` caps a
  single logical line at 256 KB, appending a visible `[logquiet: line
  truncated]` marker and discarding the remainder up to the next newline,
  so a pathological or binary-ish stream cannot grow memory without bound.

## Known limitations (stated plainly)

- Multiline grouping is a heuristic covering Python, Java/JVM, Node.js,
  and Go shapes plus a generic indentation rule - not a complete grammar
  for every language's stack-trace format (see
  [TECHNICAL_METHOD.md](TECHNICAL_METHOD.md) section 1).
- Structural normalization can, in principle, over-normalize a
  coincidentally identifier-shaped common word (see
  [TECHNICAL_METHOD.md](TECHNICAL_METHOD.md) section 2) or under-normalize
  an unrecognized variable class not in the table.
- Frequency-spike detection is wall-clock-based: it measures real elapsed
  processing time, which is the right measure for its primary use case
  (live tailing). A brand-new severe error bursting immediately is still
  caught in a fast batch replay of a static file (`cat file | logquiet`)
  via the bootstrap path, which only needs enough *events* to accumulate,
  not real elapsed time (see
  [TECHNICAL_METHOD.md](TECHNICAL_METHOD.md) section 7). The standard
  path - comparing against a pattern's own *learned* baseline - does still
  need `MinBaselineSamples` buckets (about 15 seconds by default) of real
  elapsed time to complete, so a file that finishes replaying in well
  under that window won't get standard-path detection for a pattern that
  had an established low rate and then spiked within that same short
  run. A future version could optionally use each line's own embedded
  timestamp for offline analysis so a fast replay can simulate elapsed
  time instead of measuring it in real wall-clock terms; this is not
  implemented in v0.1.
- A pattern evicted from the bounded LRU store and later reappearing is
  treated as novel again (its history is gone) - a deliberate, documented
  consequence of the bounded-memory guarantee, not a bug.
