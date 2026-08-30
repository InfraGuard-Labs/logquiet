# Technical Method

This document describes the actual algorithm LogQuiet runs, and the
reasoning behind each design choice. See [PRIOR_ART.md](PRIOR_ART.md) for
what parts of this are established technique versus specific to this
project's combination and implementation.

LogQuiet does not use machine learning, an LLM, or any external service.
Every mechanism below is deterministic and runs entirely in local memory.

## Pipeline overview

```
raw line
  -> multiline assembly       (internal/multiline)
  -> severity/prefix stripping (internal/logline, internal/severity)
  -> structural normalization  (internal/normalize)
  -> fingerprinting            (internal/fingerprint)
  -> pattern tracking + rolling-rate anomaly check (internal/pattern)
  -> render decision           (internal/render, internal/pipeline)
```

Each raw input line moves through this pipeline exactly once, in order,
with O(1) amortized work per line (bounded by a fixed number of regex/scan
passes and one hash-map lookup), and the pipeline holds no more than a
configurable bound of state regardless of stream length.

## 1. Multiline assembly

**Problem:** a single logical event (a Python traceback, a Java stack
trace, a Go panic) usually spans multiple raw lines. Treating each raw
line as an independent event breaks both readability and the
fingerprinting step below - four unrelated-looking one-line "events" per
exception, instead of one event.

**Method:** `internal/multiline.Assembler` is a small heuristic state
machine, not a per-language parser. A line continues the currently open
block if any of the following hold on its *content* (see the boundary note
below):

- It is indented by two or more spaces, or by a tab (`looksIndented`).
- It matches a Java/Node stack-frame line (`^\s*at\s+\S+\(`), a chained-
  cause line (`^\s*Caused by:`), or an elided-frames line
  (`^\s*\.\.\.\s*\d+\s*more`) - these are checked unconditionally, since
  they are distinctive enough to rarely appear in an unrelated single-line
  message.
- The block is in "Python traceback" mode (opened by a line matching
  `Traceback \(most recent call last\):`) and the line matches a `File
  "...", line N` frame, or looks like an exception summary line
  (`SomeException: message`) - the conventional unindented final line of a
  Python traceback.
- The block is in "Go panic" mode (opened by a line matching `^panic:`)
  and the line looks like a goroutine header, a bare function-call frame,
  an indented `file.go:NN` frame, or `exit status N`. A single blank line
  immediately after the panic header is also tolerated, matching Go's
  actual panic output shape.

A block is force-closed after `MaxBlockLines` (500) lines regardless of
whether it still "looks like" a continuation, which is the bounded-memory
guarantee for this stage: an adversarial or malformed stream cannot grow
one block without limit.

**A real subtlety this project got wrong once and fixed:** container
runtimes (Docker, Kubernetes) commonly prefix *every* line of raw output -
including each line of a traceback the application itself wrote as one
unindented block - with an identical timestamp (and sometimes a level
tag). If continuation detection stripped that prefix naively, a
traceback's real indentation is either destroyed (over-eager stripping,
which silently breaks grouping) or a padding artifact is misread as
indentation (under-eager stripping - e.g. many logging frameworks pad
`INFO` with a trailing space so it lines up with `ERROR`, and treating that
one leftover space as "indentation" caused unrelated lines to be merged in
early testing). The fix, encoded in `internal/severity.Detect` and
`internal/logline.Extract`, is to strip only a single separator character
after a detected level/timestamp token, never a greedy run of whitespace -
and to require **two or more** leading whitespace characters (or a single
tab) before treating a line as indented, precisely so that one leftover
alignment space cannot trigger a false continuation. This is documented
here because it is exactly the kind of bug that a spec or a demo would not
surface but a realistic fixture (`fixtures/synthetic/java-spring-exception.log`,
which uses aligned `INFO  ` padding) did.

**Known limitation:** this is intentionally not a full grammar for every
stack-trace dialect in existence (Rust panics with `RUST_BACKTRACE`,
.NET's `at ... in file:line`, Ruby backtraces, etc. are not specifically
recognized, though generic indentation-based continuation will catch some
of them). See the Limitations section of the README.

## 2. Structural normalization

**Problem:** the same underlying event produces different text every time
because of embedded variables - timestamps, IDs, IPs, durations. Exact-text
matching (what `uniq` does) cannot recognize these as the same pattern.

**Method:** `internal/normalize.Template` replaces recognized variable
classes with placeholders:

| Class | Placeholder | Example |
|---|---|---|
| ISO 8601 / syslog timestamp | `[TIMESTAMP]` | `2026-08-30T03:01:00Z` |
| Date | `[DATE]` | `2026-08-30` |
| Clock time | `[TIME]` | `03:01:00` |
| UUID | `[UUID]` | `550e8400-e29b-41d4-a716-446655440000` |
| MAC address | `[MAC]` | `00:1a:2b:3c:4d:5e` |
| IPv6 | `[IPV6]` | `fe80::1` |
| IPv4 (+ port) | `[IP]` / `[IP]:[PORT]` | `10.0.1.2:5432` |
| Memory address | `[ADDR]` | `0xc0000a4000` |
| Duration | `[DURATION]` | `5000ms`, `1.5s` |
| Byte size | `[SIZE]` | `42.5MB` |
| Percentage | `[PCT]%` | `87%` |
| Long hex hash | `[HASH]` | a git SHA or checksum |
| Generic alphanumeric ID | `[ID]` | `req-8f3ac2`, `pod-7c9f6d8b4d` |
| Bare number | `[NUM]` | `42`, `10481` |

**Design decision: no semantic, keyword-based classification of bare
numbers.** An earlier draft of this design (mirroring the illustrative
example in the original product brief) treated `User 10481 connected` as
`User [ID] connected`, inferring "this number is an ID" from the word
"User" next to it. This was deliberately dropped: keyword-based semantic
inference does not generalize across log formats and languages (the same
heuristic would need separate rules for every language's word for "user",
"id", "port", "count", ...), and a wrong guess is worse than a plain
`[NUM]` because it implies false confidence about what the number means.
Every bare number normalizes to `[NUM]` regardless of context; see the
test `normalize.TestTemplateCollapsesVariables/numeric-id-is-NUM` for the
worked example that documents this on purpose.

**Precision over recall.** Every rule requires the surrounding shape to
make a false positive unlikely: `[ID]` requires at least one letter *and*
one digit *and* a minimum length of 6, so common short technical words
("md5", "utf8", "ipv6", "b64") are not swallowed. A known, accepted
trade-off: a coincidentally identifier-shaped common word at or above that
length (e.g. "sha256") will still normalize to `[ID]`. This is documented
rather than hidden; see `normalize.TestShortWordsWithDigitsPreserved` for
what is and is not protected.

**Performance note.** An early implementation ran each of the ~17 variable
classes as an independent regex pass over every line (up to 17 full scans
per line). Benchmarking (see [BENCHMARKS.md](BENCHMARKS.md)) found this to
be the dominant cost in the whole pipeline, because Go's regexp engine has
real per-alternative cost even when nothing matches. The current
implementation splits the work: the twelve structurally intricate classes
(timestamps, UUIDs, IPs, hashes, ...) are matched by one combined,
non-capturing regex in a single pass; the five classes that profiling
showed to be expensive relative to their simplicity and that appear on a
large fraction of real lines (syslog-style month timestamps, durations,
byte sizes, generic alphanumeric IDs, and bare numbers) are hand-written
byte scanners. A single left-to-right sweep interleaves the two, so the
documented class-priority order above is preserved regardless of which
tier resolves a given span.

## 3. Fingerprinting

`internal/fingerprint.Of(severity, template)` computes an FNV-1a 64-bit
hash of the severity level plus the normalized template. Severity is
included in the hash so an INFO and an ERROR line that happen to normalize
to the same text are tracked as separate patterns - an escalation in
severity for what is nominally "the same" message is exactly the kind of
thing that should not be silently merged into routine-noise handling.

64-bit FNV-1a is a standard, extremely fast non-cryptographic hash. A
collision (two different templates hashing to the same fingerprint) is
possible in principle but not a correctness risk in the way it would be for
a security context: at the pattern cardinalities a single terminal session
realistically produces (thousands to low millions of distinct templates),
the birthday-bound collision probability is negligible, and a collision's
worst-case effect is that two genuinely different patterns share one
counter/baseline - a readability regression, not silent data loss (the
original raw lines are never discarded from the underlying log stream,
only from LogQuiet's own decorated display).

## 4. Pattern tracking, repetition, and novelty

`internal/pattern.Store` is a hash map from fingerprint to `State`,
bounded to `MaxTrackedPatterns` (default 10,000) via a least-recently-used
eviction list (`container/list`). A pattern's first-ever occurrence in a
session is always treated as **novel** and shown in full immediately, with
its real (non-normalized) values - the actual first request ID, the actual
first IP - since a first sighting is exactly when the real values are most
likely to be diagnostically useful (see the README's discussion of "why
the first occurrence isn't templated").

If a fingerprint is evicted and later reappears, it is treated as novel
again: its history is gone, so from the store's perspective it is
indistinguishable from a genuinely new pattern. This is a deliberate,
documented consequence of bounded memory (see section 6), not an oversight.

## 5. Adaptive repetition suppression (rendering)

**Problem this replaces:** naively collapsing "the same fingerprint
appearing immediately, back-to-back" understates real-world noise, because
real logs interleave multiple recurring patterns (a restart loop cycling
through half a dozen distinct messages; several services' output merged by
`docker compose logs`). A purely "consecutive-repeat" collapse measurably
fails on exactly this shape of log - see the correctness benchmark result
history in [BENCHMARKS.md](BENCHMARKS.md) for the specific regression this
caused during development (0% suppression on a Kubernetes restart-loop
fixture) and the fix.

**Method:** every occurrence after the first is *accumulated* into a
per-fingerprint counter (`internal/render.Renderer.pending`), independent
of what other patterns arrive in between. Accumulated counters are flushed
- as a compact "`template` / `× N`" summary line - on a short, bounded
cadence per pattern (`FlushInterval`, default 2s; `ProtectedFlushInterval`,
default 500ms, for severities at or above `-severity-protect`, default
ERROR), rather than either reprinting every occurrence or waiting
indefinitely. This correctly handles both back-to-back repeats and
interleaved recurring patterns with the same mechanism.

**Deliberate simplification versus cursor-repositioning redraw.** An
earlier version tried to redraw a single "active" counter line in place
using ANSI cursor movement, mimicking a progress-bar-style live update.
This was dropped in favor of periodic summarization for two reasons: (1)
it only works cleanly for one concurrently-updating line, and real logs
routinely have several patterns recurring at once, which would require
tracking and repositioning multiple independent cursor targets scattered
through scrollback - fragile across terminal emulators, multiplexers
(tmux/screen), and window resizing; (2) periodic summarization is trivial
to reason about and test deterministically (see `render.TestAccumulateFlushesOnInterval`),
whereas cursor-position bugs are notoriously hard to cover with automated
tests. The cost is a small, bounded, configurable delay before a count
visibly updates - judged an acceptable trade for correctness and
robustness.

## 6. Bounded memory

Three independent mechanisms bound memory regardless of stream length:

1. **Pattern store LRU eviction** (`pattern.Store`, default 10,000
   patterns) - bounds the number of tracked templates.
2. **Multiline block cap** (`MaxBlockLines`, 500) - bounds a single
   in-flight event's size.
3. **Line length cap** (`internal/reader.MaxLineBytes`, 256 KB) - an
   unusually long or malformed (binary-ish, newline-free) line is
   truncated with a visible marker rather than buffered without limit;
   see `reader.TestOverlongLineIsTruncatedNotUnbounded`.

Each pattern's own rolling-rate state (section 7) is a small fixed-size
ring buffer (12 buckets by default), not a growing list of timestamps, so
per-pattern memory is O(1) regardless of how many times that pattern has
recurred.

## 7. Frequency-spike (anomaly) detection

**The critical safety property this exists for:** repetition is not
automatically noise. An error occurring far more often than its own
historical baseline is itself the most important signal in the stream, and
must not be quietly absorbed into "just another repeat counter."

### History: the first version was too slow to be useful, and this was found and fixed

The first shipped version gated all detection behind a flat, unconditional
`now.Sub(FirstSeen) >= 2 minutes` timer, checked before anything else in
`State.Record`. That meant no anomaly of any kind - however severe, however
far outside any reasonable baseline - could ever be flagged during a
pattern's first two minutes of existence, and a brand-new pattern (the
common case for "a severe error just started happening") had no path to
detection at all, ever, since it never accumulates a "prior" baseline to
be compared against. Manual testing against realistic short sessions (a
rapid burst of a brand-new ERROR, and an established low-rate pattern
followed by a sudden burst) reported zero anomalies in both cases - this
was reported, reproduced, root-caused, and is fixed as described below.
The redesign also found and fixed a second, subtler issue during
validation: with the original 60-second current-rate window, a burst that
had only just started was diluted by the quiet time still sitting in the
rest of that same fixed-length window, making detection marginal and
inconsistent from run to run - not a rare edge case, but the normal
shape of "a spike just started."

### Current method (`internal/pattern.State`)

Each pattern keeps a ring buffer of fixed-width time buckets (default: 3
buckets × 5s = a **15-second** "current rate" window - short and
deliberately so; see the rationale in the package doc comment in
`internal/pattern/pattern.go`, summarized below). Every time a bucket
finishes accumulating - starting from the very first one, not only once a
full window has cycled - its rate is folded into a slow exponentially
weighted moving average that represents the pattern's learned "normal"
rate:

```
baseline = alpha * observed_bucket_rate + (1 - alpha) * baseline   (alpha = 0.05 by default)
```

Baseline quality does not depend on the current-rate window's length: it
is a lifetime average over every bucket ever folded, not a function of how
many buckets are held in the ring at once. Shrinking the window (from an
original 60s to 15s) therefore made burst detection dramatically faster
and more reliable without making the learned baseline any less
representative.

**Standard path - a pattern with an established baseline.** Once at least
`MinBaselineSamples` buckets (default 3, i.e. about 15 seconds of real
elapsed history) have been folded in, the baseline is trusted, and a spike
is flagged when **all** of the following hold:

- The current 15-second-window rate exceeds the baseline by at least
  `SpikeMultiplier` (default 8x for ordinary severities, `ProtectMultiplier`
  = 3x for severities at or above `-severity-protect`, default ERROR - a
  rare error class is treated as newsworthy at a lower bar than routine
  chatter).
- The current window contains at least `MinWindowEvents` (default 5)
  events - so a baseline near zero cannot be "exceeded" by one stray
  occurrence.
- At least `Cooldown` (default 60s) has passed since the last alert for
  this exact pattern - so a sustained spike produces periodic alerts, not
  one per occurrence.

**Bootstrap path - a pattern with no trustworthy baseline yet.** This
covers exactly the case the standard path structurally cannot: a pattern
that is brand new, or too young to have `MinBaselineSamples` of real
history, but is already firing at a severe, sustained rate - "a rapid
burst of repeated ERROR database timeout events" reported during manual
testing is precisely this shape. This path applies **only** to severities
at or above `-severity-protect` (default ERROR); an ordinary INFO/DEBUG
pattern with no baseline yet is never bootstrap-flagged, however high its
absolute rate, which is what keeps a routine high-frequency INFO stream
from ever producing a false positive (see the worked scenarios below). For
a protected severity with no trustworthy baseline, an assumed baseline
(`AssumedBaselinePerMin`, default 1.0/min - "we assume a brand-new error
class would not normally recur more than about once a minute") stands in
for the missing real one, and a spike fires once the window holds at least
`MinBootstrapEvents` (default 10, deliberately higher than the standard
path's `MinWindowEvents`, so a merely-new-but-modest error occurring a
handful of times is not elevated to a spike banner) events at a rate
exceeding `AssumedBaselinePerMin * multiplier`. The rendered banner labels
this case explicitly as "(new pattern)" with "no prior history", rather
than presenting the assumed number as if it had been measured - see
`internal/render.Renderer.EmitAnomaly`.

If a bootstrap-flagged pattern's elevated rate persists, the standard path
naturally takes over once real history accumulates: the baseline EWMA
converges toward the new steady rate, and once `current < baseline *
multiplier` holds against that now-accurate baseline, alerts stop - a
permanently-elevated-but-stable rate becomes "the new normal" rather than
alerting forever (verified by
`pattern.TestF_LongRunningStreamBaselineEventuallyAdapts`).

### Worked scenarios (each has an automated test in `internal/pattern/pattern_test.go`)

| Scenario | Path | Outcome |
|---|---|---|
| A: stable routine pattern, indefinitely | - | No baseline ever exceeded; never alarms |
| B: established low-rate pattern, then a sudden large increase | Standard | Flagged, using the pattern's own real learned baseline |
| C: brand-new severe ERROR/CRITICAL, immediate burst, no history | Bootstrap | Flagged within seconds, labeled "(new pattern)" |
| C (variant): a new error type occurs only a handful of times | Bootstrap (does not fire) | Below `MinBootstrapEvents`; shown via ordinary novelty/severity display, not elevated to a spike |
| D: routine high-frequency INFO from the very start | Neither | Bootstrap never applies to ordinary severities; once a baseline forms it matches the pattern's own steady rate |
| E: short-lived stream, ordinary-severity burst | Neither | Correctly not flagged - too little data, not a protected severity |
| E: short-lived stream, severe burst | Bootstrap | Still flagged even in a stream only a few seconds long |
| F: long-running stream, sustained spike | Standard, cooldown-bounded | Periodic alerts, not one per occurrence |
| F: long-running stream, permanent rate change | Standard, self-correcting | Alerts stop once baseline converges to the new rate |

This is a rolling-baseline-ratio method with an EWMA baseline, plus a
severity-gated bootstrap fallback for the no-baseline case - both are
well-established, explainable statistical process-control techniques (see
[PRIOR_ART.md](PRIOR_ART.md) section 3), deliberately chosen over
heavier alternatives (Holt-Winters, CUSUM, z-score) for a first version:
easy to explain, easy to tune, and its failure modes are easy to reason
about. It is not machine learning and does not claim to be
state-of-the-art anomaly detection - see the README's Limitations section
for when it will and won't catch a real incident (e.g., a slow multi-hour
ramp may never cross the instantaneous multiplier threshold at any single
point, since the baseline adapts alongside it).

## 8. Severity awareness

`internal/severity.Detect` recognizes common level tokens (TRACE, DEBUG,
INFO, NOTICE, WARN/WARNING, ERROR/ERR, CRITICAL/CRIT/ALERT,
FATAL/PANIC/EMERG) in both bracketed (`[ERROR]`) and bare (`ERROR:`,
`ERROR `) forms, tolerating an arbitrary prefix (timestamp, hostname, pid -
as in syslog/journalctl output) before the level token, and rejecting
false positives like a path segment (`/info`) or a qualified class name
(`com.foo.Info`). A line with no recognizable level is treated as
`Unknown`, which ranks equivalently to `Info` for suppression purposes
(neither specially protected nor specially suppressed) - this is the "do
not assume every log format uses brackets" requirement, satisfied by
degrading gracefully rather than guessing.

## 9. What is deterministic and what is a documented heuristic

For evidentiary honesty, this table separates what is exact/deterministic
from what is a tuned heuristic with known edge cases:

| Component | Deterministic? | Notes |
|---|---|---|
| Fingerprinting | Yes | FNV-1a hash of severity + template |
| Suppression / flush cadence | Yes | Pure function of elapsed wall-clock time and configured intervals |
| Severity detection | Heuristic | Token/shape-based; see section 8 |
| Structural normalization | Heuristic | Precision-favoring pattern classes; see section 2 |
| Multiline grouping | Heuristic | Line-shape based, not a per-language grammar; see section 1 |
| Frequency-spike detection | Statistical, deterministic given history | Threshold-based; not ML |

No component uses randomness, external data, or a trained model.
