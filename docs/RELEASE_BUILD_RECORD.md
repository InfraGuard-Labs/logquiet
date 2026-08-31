# Release Build Record

This file records the facts of an actual local build of the `v0.1.0`
release artifacts, so the relationship between a specific source commit
and a specific set of binaries is explicit and checkable - not a signing
or provenance claim, just a factual record of what was built, from what,
and when. Regenerate this file (do not hand-edit the hashes) whenever the
artifacts in `dist/` are rebuilt from source, using
`scripts/build-release.sh` followed by re-running the checks in
[RELEASE_PROCESS.md](RELEASE_PROCESS.md).

## v0.1.0 rebuild

| Field | Value |
|---|---|
| Version | `v0.1.0` |
| Source commit | `c08b930bac33a35cc0b55006c57a1c09cd29e769` |
| `git describe --tags --always` | `v0.1.0-2-gc08b930` |
| Build date/time (UTC) | `2026-08-31T01:19:36Z` |
| Go version | `go1.27.0 windows/amd64` |
| Build command | `scripts/build-release.sh v0.1.0` |
| Build flags | `CGO_ENABLED=0`, `-trimpath -ldflags "-s -w -X main.version=v0.1.0"` |
| Target platforms | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 |

### Artifacts and SHA256 hashes

```
2e62865d2771adbcd8b689ceeeee5bb7aa994a1c89c65f74709b97a689104c06  logquiet-v0.1.0-darwin-amd64
5126f23a8e6bc28c78654c7690ac7ab582058ac6dd3588817e42db16dcc8dad1  logquiet-v0.1.0-darwin-arm64
e33f726b1d6d8d17e64ae656e9ddf008e1ef40baebd0d86dc07d36c88c24081e  logquiet-v0.1.0-linux-amd64
47dc6dac1d380a841c5d1fa3fe804ca4f15d67b862a6e43c8d56ec86e64b11b0  logquiet-v0.1.0-linux-arm64
3d2fbbf96f68000c1aae457fdc4a808d22ebc1924dc512d34ef7b51596cb0b4c  logquiet-v0.1.0-windows-amd64.exe
```

(Identical to `dist/SHA256SUMS.txt` at the time of this build; that file,
not this one, is the source of truth if they ever diverge - this table is
a copy for reviewability alongside the commit it was built from.)

### Why this rebuild happened

The `v0.1.0` tag was cut at commit `c08b930bac33a35cc0b55006c57a1c09cd29e769`.
Binaries previously present in `dist/` had been built from an earlier
point in `main` and were stale relative to that commit - missing numeric
CLI flag validation, `--color`, the dual suppression metrics
(`logical_event_suppression_percentage` / `raw_line_suppression_percentage`),
impact-report schema v2, and the current stats wording (`error events
observed` / `warning events observed`). This rebuild replaces those stale
binaries with ones built directly from the commit above, with no source
changes and no version bump - see [RELEASE_PROCESS.md](RELEASE_PROCESS.md)
for the standing build/release checklist this followed.
