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

## Schema (v1)

```json
{
  "logquiet_version": "v0.1.0",
  "generated_at": "2026-08-30T12:00:00Z",
  "session_duration_seconds": 42.1,
  "raw_lines": 100000,
  "displayed_events": 37,
  "suppressed_events": 99963,
  "suppression_percentage": 99.96,
  "structural_patterns": 12,
  "warning_events": 3,
  "error_events": 5,
  "anomaly_events": 1,
  "processing_rate_lines_per_second": 24500.2
}
```

| Field | Type | Meaning |
|---|---|---|
| `logquiet_version` | string | The version of the LogQuiet binary that produced the report |
| `generated_at` | string (RFC 3339, UTC) | When the report was written |
| `session_duration_seconds` | number | Wall-clock time from process start to report generation |
| `raw_lines` | integer | Total input lines read |
| `displayed_events` | integer | Distinct events shown in full (novel patterns + anomaly banners) |
| `suppressed_events` | integer | Occurrences collapsed into repeat counters |
| `suppression_percentage` | number | `100 * suppressed / (suppressed + displayed)` |
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

This is schema version 1 (implicit - no `schema_version` field is emitted
yet, since there has only ever been one shape). If a future version needs a
breaking change, a `schema_version` field will be added and this document
updated accordingly; existing fields will not change meaning silently.

## Example: generating one

```bash
kubectl logs -f deployment/api | logquiet --impact-report impact.json
# ... let it run, then Ctrl+C ...
cat impact.json
```
