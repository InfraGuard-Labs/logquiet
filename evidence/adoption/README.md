# Monthly Adoption Metrics Ledger

`monthly-metrics.csv` in this directory is a header-only template. It has
no data rows because none have been collected yet - see
[evidence/README.md](../README.md) "The governing rule": an empty ledger
is more honest than a prepopulated one.

## Adding a snapshot

1. Run [scripts/snapshot-public-metrics.sh](../../scripts/snapshot-public-metrics.sh)
   for the public GitHub numbers (stars, forks, open issues, release
   download counts), or pull them manually from the GitHub UI/API if you
   prefer.
2. Review any voluntary submissions received since the last snapshot
   (impact reports, feedback-form/issue-template responses) against the
   "verified" standard in
   [docs/METRICS_DEFINITIONS.md](../../docs/METRICS_DEFINITIONS.md)
   before counting them.
3. Append one row to `monthly-metrics.csv` - do not edit past rows except
   to correct a factual error (and note the correction in that row's
   `notes` field).
4. Leave a field blank (not zero) if it was not checked this cycle; use
   `0` only when you actually confirmed the count is zero.
5. Set `evidence_source` to where the row's numbers came from (a script
   output file, an API response, a specific person's confirmation) - see
   [evidence/README.md](../README.md) for how to preserve the underlying
   artifact (screenshot, saved JSON, etc.) alongside the row.

## Column reference

Every column is defined precisely in
[docs/METRICS_DEFINITIONS.md](../../docs/METRICS_DEFINITIONS.md). Do not
add a column without also adding its definition there.
