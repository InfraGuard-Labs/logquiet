# Release Build Record

This file records the facts of an actual local build of the `v0.1.0`
release artifacts, so the relationship between a specific source commit
and a specific set of binaries is explicit and checkable - not a signing
or provenance claim, just a factual record of what was built, from what,
and when. Regenerate this file (do not hand-edit the hashes) whenever the
artifacts in `dist/` are rebuilt from source, using
`scripts/build-release.sh` followed by re-running the checks in
[RELEASE_PROCESS.md](RELEASE_PROCESS.md).

## v0.1.0 - authoritative build (from the exact tagged commit)

The `v0.1.0` annotated tag object is `742a7b7cde59e50159832552088b4e17d45dc44f`;
the commit it points at - the actual release source - is
`676aee286ce3a3b1a0c0494ff92c08e937a35d77` (`chore: migrate repository
path to InfraGuard-Labs/logquiet`). This is the commit these binaries were
built from, in a clean detached worktree checked out directly at the
`v0.1.0` tag (`git worktree add --detach <path> v0.1.0`), not on `main`.

| Field | Value |
|---|---|
| Version | `v0.1.0` |
| Tag object (annotated) | `742a7b7cde59e50159832552088b4e17d45dc44f` |
| Tagged commit (authoritative source) | `676aee286ce3a3b1a0c0494ff92c08e937a35d77` |
| `git describe --exact-match --tags HEAD` (in the tag worktree) | `v0.1.0` |
| Build date/time (UTC) | `2026-08-31T01:26:10Z` |
| Go version | `go1.27.0 windows/amd64` |
| Build command | `scripts/build-release.sh v0.1.0` |
| Build flags | `CGO_ENABLED=0`, `-trimpath -ldflags "-s -w -X main.version=v0.1.0"` |
| Target platforms | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 |

### Artifacts and SHA256 hashes (authoritative - these are what `dist/` contains)

```
b4f5c09f54bb3211236a57776c08d238155d142ea5c0592c39e26e5dc091fd03  logquiet-v0.1.0-darwin-amd64
062164b8f6f9fd644b968b7df9c5670f39bd2dcc48e598eeaea407d8beef63cc  logquiet-v0.1.0-darwin-arm64
3c2bb4e89b0d2f8c9b4a5238aa7f5a37cbe61dadb837ab1b47110c471d8baf76  logquiet-v0.1.0-linux-amd64
600e9e7476aedcb3474b1c9ec357eff52d529e7334803be822a222af60150ad9  logquiet-v0.1.0-linux-arm64
e2bfcb177c35eb63954085ca60db786a785ef4445550223432f8460fd70cc30a  logquiet-v0.1.0-windows-amd64.exe
```

(Identical to `dist/SHA256SUMS.txt` at the time of this build; that file,
not this one, is the source of truth if they ever diverge.)

## Superseded build (do not use) - built from c08b930

An earlier rebuild pass used commit `c08b930bac33a35cc0b55006c57a1c09cd29e769`
(HEAD of `main` at the time, two documentation-only commits after the
`v0.1.0` tag) rather than the exact tagged commit. That commit was
**mischaracterized at the time as the tag's source** - it was not; the
`v0.1.0` tag has always pointed at `676aee2`. A dedicated provenance
verification (rebuilding from a clean `git worktree add --detach v0.1.0`
checkout) found that every one of the five artifacts differs, byte for
byte, from the ones built directly at the tag:

| Artifact | c08b930-built SHA256 (superseded) | Exact-tag-built SHA256 (authoritative) | Identical? |
|---|---|---|---|
| `logquiet-v0.1.0-darwin-amd64` | `2e62865d...9104c06` | `b4f5c09f...091fd03` | no |
| `logquiet-v0.1.0-darwin-arm64` | `5126f23a...16dcc8dad1` | `062164b8...eaea407d8beef63cc` | no |
| `logquiet-v0.1.0-linux-amd64` | `e33f726b...b88c24081e` | `3c2bb4e8...b47110c471d8baf76` | no |
| `logquiet-v0.1.0-linux-arm64` | `47dc6dac...c8d56ec86e64b11b0` | `600e9e74...b822a222af60150ad9` | no |
| `logquiet-v0.1.0-windows-amd64.exe` | `3d2fbbf9...34ef7b51596cb0b4c` | `e2bfcb17...432f8460fd70cc30a` | no |

**Root cause, confirmed with `go version -m` on both binary sets:** Go
automatically embeds VCS stamping (`vcs.revision`, `vcs.time`,
`vcs.modified`) into every binary built inside a clean git checkout, and
this is included in the compiled output regardless of `-trimpath` (which
only strips filesystem paths, not VCS metadata). The c08b930-built
binaries embed `vcs.revision=c08b930bac33a35cc0b55006c57a1c09cd29e769`;
the exact-tag-built binaries embed
`vcs.revision=676aee286ce3a3b1a0c0494ff92c08e937a35d77`. That single
embedded field is sufficient to change every byte's hash even though the
two commits differ only in unrelated documentation files (`fc9c3bd`,
`c08b930`) that do not touch `cmd/`, `internal/`, or `go.mod`. The
application logic, flag behavior, JSON/impact-report schema, and stats
wording are identical between the two commits - only the self-reported
build provenance differs, and the c08b930 build's self-reported provenance
was wrong for a `v0.1.0` release (it named a commit two steps past the
tag as the source). This is why byte-identity cannot be assumed from
"the intervening commits are documentation-only" and must be verified
directly, which is what this pass did.

The c08b930-built binaries and their `SHA256SUMS.txt` have been discarded
from `dist/` and must not be uploaded to the GitHub Release.

### Behavioral verification at the exact tagged commit

Confirmed on the native (`windows-amd64`) exact-tag binary, matching the
required v0.1.0 feature set:

- `--version` -> `logquiet v0.1.0`
- `--color` recognized (no "flag provided but not defined" error)
- `--max-patterns 0` and `--spike-multiplier -1` both rejected, exit code 2
- Impact report: `schema_version: 2`, both
  `logical_event_suppression_percentage` and
  `raw_line_suppression_percentage` present, obsolete
  `suppression_percentage` absent
- Stats output uses `warning events observed:` / `error events observed:`

All required v0.1.0 fixes were genuinely present at the tagged commit
itself, not only on later `main` commits.
