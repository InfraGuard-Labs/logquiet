# Impact Report Schema

`logquiet --impact-report <path>` writes a single JSON object to `<path>`
when the input stream ends (EOF, Ctrl+C, or SIGTERM). It is entirely
optional - nothing is written unless you pass this flag - and it never
contains raw log content. See [PRIVACY.md](PRIVACY.md) for the privacy
rationale; this document is the field-by-field technical reference.

## Purpose

The report exists so a user can voluntarily generate and share aggregate,
sanitized evidence of LogQuiet's effect on a real log stream (for their own
records, for a team retro, or as evidence supporting this project's actual,
non-fabricated impact - see [evidence/README.md](../evidence/README.md)) -
without ever having to share the underlying log content to make that case.

## Physical lines vs. logical events - read this before quoting a percentage

LogQuiet's pipeline has two different units of "how much", and the report
exposes both rather than picking one and hiding the difference:

- **Physical input lines** - one per line actually read from the stream.
  `raw_lines`, `displayed_raw_lines`, and `suppressed_raw_lines` are all in
  this unit, and `displayed_raw_lines + suppressed_raw_lines == raw_lines`
  always holds exactly.
- **Logical events** - after multiline assembly, so a single multi-line
  stack trace or traceback is *one* logical event, not one per raw line it
  spans. `displayed_events` and `suppressed_events` are in this unit.

These give two **legitimately different** suppression percentages, and
picking the wrong one for a claim is a real, not merely pedantic, mistake:
a stream containing one collapsed multi-line block plus several collapsed
single-line repeats will show a *higher* logical-event suppression
percentage than raw-line suppression percentage, because that one
multi-line block's several raw lines only count once on the logical-event
side. For example, against this project's own `java-spring-exception.log`
fixture: `logical_event_suppression_percentage` is 99.13%, while the true
`raw_line_suppression_percentage` is 96.62% - both real, both computed
from the same run, describing different things.

**If you want to claim "N% of my raw log lines were hidden", the correct
field is `raw_line_suppression_percentage`.** `logical_event_suppression_percentage`
answers a different, also-useful question: "of the distinct structural
things that happened, what fraction were routine repeats rather than novel
occurrences." Neither one is "the suppression number" on its own.

## Schema (v2)

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

| Field | Type | Meaning |
|---|---|---|
| `schema_version` | integer | Currently `2`. See "Versioning" below. |
| `logquiet_version` | string | The version of the LogQuiet binary that produced the report |
| `generated_at` | string (RFC 3339, UTC) | When the report was written |
| `session_duration_seconds` | number | Wall-clock time from process start to report generation |
| `raw_lines` | integer | Total **physical** input lines read |
| `displayed_events` | integer | Distinct **logical events** shown in full (novel patterns + anomaly banners) |
| `suppressed_events` | integer | **Logical events** collapsed into repeat counters |
| `logical_event_suppression_percentage` | number | `100 * suppressed_events / (suppressed_events + displayed_events)` - a logical-event ratio, not a line-count ratio |
| `raw_line_suppression_percentage` | number | `100 * suppressed_raw_lines / raw_lines` - the true fraction of physical lines not individually displayed |
| `displayed_raw_lines` | integer | Physical lines belonging to a displayed logical event (a multi-line event's lines all count here) |
| `suppressed_raw_lines` | integer | Physical lines belonging to a suppressed logical event. `displayed_raw_lines + suppressed_raw_lines == raw_lines` always. |
| `structural_patterns` | integer | Distinct structural fingerprints tracked at report time |
| `warning_events` | integer | Occurrences **observed** at WARN severity in the input - every occurrence counts once, whether it was shown individually or collapsed into a suppressed repeat counter. Not a count of individually-displayed WARN lines. |
| `error_events` | integer | Occurrences **observed** at ERROR severity or above in the input, with the same "observed, not necessarily individually displayed" meaning as `warning_events`. |
| `anomaly_events` | integer | Frequency-spike banners raised |
| `processing_rate_lines_per_second` | number | `raw_lines / session_duration_seconds` |

## What is deliberately excluded

The report contains **no**:

- Raw log lines, message text, or normalized templates.
- Hostnames, IP addresses, usernames, request/trace IDs, or any other
  value observed in the input.
- File paths, environment details, or anything identifying the machine or
  user beyond what you choose to share about the report file itself.

This is enforced structurally, not by redaction: look at
`internal/stats.ImpactReport` and `internal/stats.BuildImpactReport` in the
source - the struct has no field capable of holding a log line, and no
code path passes one in. The schema in this document and the struct
definition in code are the same source of truth; if they ever diverge,
that is a bug.

## Versioning

This is schema version 2. Version 1 (undocumented as such, since no
`schema_version` field existed yet) had a single `suppression_percentage`
field computed the same way `logical_event_suppression_percentage` is now
- it was replaced, not merely renamed, because that one field could be
(and was) read as a raw-line reduction claim it did not actually support.
`schema_version` was added at the same time so a future breaking change
has a field to bump rather than requiring readers to diff the whole
schema. Existing fields will not change meaning silently going forward.

## Example: generating one

```bash
kubectl logs -f deployment/api | logquiet --impact-report impact.json
# ... let it run, then Ctrl+C ...
cat impact.json
```
