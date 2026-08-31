# Privacy

LogQuiet is built on one non-negotiable premise: **it will frequently
process production logs, which routinely contain sensitive data** (IP
addresses, user identifiers, session tokens, hostnames, business data).
Treating that seriously means being specific, not just asserting "we care
about privacy."

## What LogQuiet does

- Reads log data from stdin or a local file you name.
- Processes it entirely in local process memory.
- Writes human- or machine-readable output to stdout, and diagnostics to
  stderr.
- Optionally, only if you pass `--impact-report <path>`, writes a small
  JSON file of **aggregate statistics only** (see below) to a path you
  choose.

That is the complete list of I/O LogQuiet performs.

## What LogQuiet does not do

- **No network calls, ever**, during normal operation - not for updates,
  not for telemetry, not for anything. There is no code path in this
  repository that opens a network connection.
- **No telemetry, by default or otherwise.** There is no opt-out flag for
  telemetry because there is no telemetry to opt out of.
- **No account, license server, or authentication of any kind.**
- **No raw log content ever leaves the process**, in any mode, including
  `--json` (which is a local stdout stream, not a transmission) and
  `--impact-report`.
- **No persistence of log content.** LogQuiet does not write your logs, or
  any derivative of their content, to disk. Structural templates and
  counts exist only in memory for the lifetime of the process.

## The `--impact-report` file, exactly

**Impact reports are generated locally and are never uploaded
automatically.** LogQuiet has no code path that transmits one anywhere -
see "No network calls, ever" above. If you choose to share one (for
example, attaching it to a GitHub issue or the feedback form in
[docs/ADOPTER_FEEDBACK_FORM.md](ADOPTER_FEEDBACK_FORM.md)), that is a
separate, manual, entirely optional action you take yourself.

Running `logquiet --impact-report report.json` writes a single JSON object
containing only:

```json
{
  "schema_version": 2,
  "logquiet_version": "v0.1.0",
  "generated_at": "2026-08-30T12:00:00Z",
  "session_duration_seconds": 42.1,
  "raw_lines": 100000,
  "displayed_events": 37,
  "suppressed_events": 99963,
  "logical_event_suppression_percentage": 99.96,
  "raw_line_suppression_percentage": 99.94,
  "displayed_raw_lines": 61,
  "suppressed_raw_lines": 99939,
  "structural_patterns": 12,
  "warning_events": 3,
  "error_events": 5,
  "anomaly_events": 1,
  "processing_rate_lines_per_second": 24500.2
}
```

Every field is a count, a percentage, a duration, or a version string. The
full schema is documented field-by-field in
[IMPACT_REPORT.md](IMPACT_REPORT.md). This report **never contains**:

- Raw log lines or any substring of them.
- Hostnames, IP addresses, usernames, or any other value seen in the input.
- File paths, secrets, tokens, or credentials.
- Anything that would let a reader reconstruct what was actually logged.

Generating this file is entirely your choice - it is not written unless
you pass the flag - and sharing it is entirely your choice too.

## Terminal output and color

LogQuiet's normal terminal output does display real log content (that is
the point of the tool), including the first occurrence of a newly-seen
pattern shown with real values. This is local terminal output under your
control, equivalent to running `cat` or `tail` on the same file; LogQuiet
adds structure and suppression on top, not exposure beyond what the raw
log already contains on your own screen.

## Your responsibility

LogQuiet cannot know whether a given log line contains something
sensitive - that judgment belongs to whoever configured the upstream
logging. If your logs contain secrets that should not be displayed, that
is true whether or not LogQuiet is in the pipeline; LogQuiet does not
increase the exposure of a log stream you already have local access to,
and does not send it anywhere new.

## Questions

If you believe LogQuiet's behavior does not match this document, please
open an issue - a mismatch between documentation and code is a bug we want
to know about immediately.
