# Demo assets: how they were made, and how to remake them

Everything under [`assets/demo/`](../assets/demo/) is a **real, captured
`logquiet` execution** against a **synthetic, fabricated** fixture -
never real production, proprietary, confidential, or personally
identifiable data. Nothing here alters or fakes application behavior; the
only thing this tooling adds on top of real captured output is a terminal-
window visual frame (title bar, font, background) for a clean screenshot.

## Reproduce everything yourself

```bash
# 1. (Re)generate the deterministic fixtures these demos run against.
demo/generate-noisy-log.sh

# 2. Reproduce each README screenshot exactly.
demo/run-before.sh            # assets/demo/before.png
demo/run-after.sh             # assets/demo/after.png
demo/run-frequency-spike.sh   # assets/demo/frequency-spike.png
demo/run-stack-trace.sh       # assets/demo/stack-trace.png
```

Requires the Go toolchain (already required to build LogQuiet) and a
local Chromium-based browser (Google Chrome or Microsoft Edge - both
common defaults; no other install needed, and no external or hosted
service is used for anything here).

## How each piece works

```
fixture (demo/fixtures/*.log, deterministic - see tools/fixturegen/demo.go)
  -> logquiet --color <fixture>      real logquiet execution, real ANSI codes
  -> demo/ansi2html                  translates the SAME SGR codes render.go emits into HTML spans
  -> demo/gen-screenshot             wraps the HTML in a plain terminal-window template
  -> headless Chrome/Edge --screenshot   renders it to a PNG, entirely locally
```

- **`tools/fixturegen/demo.go`** generates the three fixtures under
  `demo/fixtures/` from fixed templates and a fixed synthetic timestamp
  base (never `time.Now()`, never unseeded randomness) - running it twice
  produces byte-identical files.
- **`demo/ansi2html`** is a literal translator for exactly the SGR codes
  `internal/render/render.go` emits (reset, bold, dim, yellow, red) - it
  makes no rendering decisions of its own; the color you see in a
  screenshot is the color LogQuiet's own renderer chose.
- **`demo/gen-screenshot`** only supplies chrome around that real content:
  a dark title bar with the traffic-light dots convention, a monospace
  font, and the literal command line being demonstrated. It never edits
  the captured text.
- **`--color`** (see `logquiet --help`) forces the same ANSI codes a real
  attached terminal would produce, even though these captures run in a
  non-interactive pipe - this is a genuine, generally useful flag (for
  piping through `less -R` or saving colored output to a file), not a
  demo-only hack; see `internal/config/config.go`.

## What each fixture demonstrates

| Fixture | Demonstrates | Script |
|---|---|---|
| `noisy-app.log` | Routine DB/health-check noise collapsing while a critical failure and its traceback survive | `run-before.sh` / `run-after.sh` |
| `frequency-spike.log` | A brand-new error class bursting with zero prior history, caught by the bootstrap anomaly path (docs/TECHNICAL_METHOD.md section 7) | `run-frequency-spike.sh` |
| `stack-trace.log` | A full multiline Python traceback preserved while surrounding routine lines collapse | `run-stack-trace.sh` |

## The frequency-spike screenshot is deterministic despite wall-clock-based detection

Frequency-spike detection measures real elapsed processing time (see
docs/TECHNICAL_METHOD.md section 7), which sounds like it should make a
screenshot script flaky. It doesn't, for `frequency-spike.log`
specifically: the scenario is a **brand-new** error class bursting
immediately, which uses the *bootstrap* path - that path only needs enough
**events** to accumulate (`MinBootstrapEvents`, default 10), not real
elapsed time, so reading a small static file (which finishes in
milliseconds) still reliably crosses that threshold every single run. This
was verified by running the capture repeatedly before relying on it for a
screenshot - see the development history in `docs/BENCHMARKS.md` and
`docs/TECHNICAL_METHOD.md` for the story of how the frequency-spike
detector's timing behavior was found to be broken and fixed.

## Animated demo

An animated terminal recording (noisy stream -> `logquiet` -> condensed
output) was considered but not produced automatically: this environment
has no local, reliable, non-hosted GIF/terminal-recording toolchain
(no `asciinema`, `ttygif`, `ffmpeg`, or similar available), and installing
one was out of scope for an automated pass rather than risk a poor-quality
or flaky result. Two reproducible options if you want to make one:

1. **Screen-record the real thing.** Run `docker compose logs -f | logquiet`
   (or replay `demo/fixtures/noisy-app.log` slowly, e.g.
   `while read -r l; do echo "$l"; sleep 0.15; done < demo/fixtures/noisy-app.log | logquiet`)
   in a terminal sized to about 100x30, and record it with any local
   screen recorder, then convert to GIF with `ffmpeg` if you have it:
   `ffmpeg -i recording.mp4 -vf "fps=12,scale=900:-1" assets/demo/demo.gif`.
2. **Frame-stitch from screenshots.** `demo/capture-screenshot.sh` can be
   called repeatedly against a growing prefix of a fixture (e.g. the first
   5, 15, 30, and all lines) to produce a handful of PNG frames, which
   `ffmpeg -framerate 1 -i frame%d.png assets/demo/demo.gif` (or any GIF
   encoder) can assemble into a slow-motion animation without needing a
   real terminal recorder at all.

Either path is fully local and reproducible; neither depends on a hosted
service.
