# Release Process

## Versioning

LogQuiet follows [Semantic Versioning](https://semver.org/). Until 1.0.0,
minor version bumps (0.x.0) may include breaking changes to flags or the
JSON/impact-report schema; such changes will always be called out in
[CHANGELOG.md](../CHANGELOG.md).

## Why this is not yet 1.0.0

A 1.0.0 tag is a claim of stability that this project has not yet earned
through real-world use. The honest state as of this writing:

- The core pipeline is implemented, tested, fuzzed, and benchmarked
  end-to-end (see [BENCHMARKS.md](BENCHMARKS.md)).
- It has not yet been used against real, unmodified production traffic by
  anyone other than its author, against synthetic fixtures.
- The normalization and multiline-grouping heuristics are known to have
  edge cases (documented in [ARCHITECTURE.md](ARCHITECTURE.md) and
  [TECHNICAL_METHOD.md](TECHNICAL_METHOD.md)) that only broader real-world
  use will fully surface.

The initial public release is tagged `v0.1.0` accordingly. The milestone
for `v1.0.0` is: a period of real external usage with no critical defects
reported, plus whatever flag/schema changes that usage motivates, made
*before* the API is declared stable.

## Artifact/source traceability

Published binaries must be built from the exact commit the version tag
points at - not from an earlier or later point on `main` - otherwise the
published artifact silently drifts from the source it claims to represent.
[RELEASE_BUILD_RECORD.md](RELEASE_BUILD_RECORD.md) records, for each real
local build of the release artifacts, the source commit, Go version, build
flags, target platforms, and SHA256 hashes actually produced, so that
relationship is checkable rather than assumed.

## Release checklist

1. Ensure `main` is green: `go build ./...`, `go vet ./...`, `go test
   ./...`, `gofmt -l .` (empty output).
2. Update [CHANGELOG.md](../CHANGELOG.md) with the new version's changes.
3. Re-run the correctness benchmark suite and confirm no known-important
   event is missed: `go run ./benchmarks/correctness`.
4. Tag: `git tag -a vX.Y.Z -m "vX.Y.Z"` and push the tag.
5. The release workflow (`.github/workflows/release.yml`) triggers on the
   tag push and:
   - Cross-compiles for linux/amd64, linux/arm64, darwin/amd64,
     darwin/arm64, windows/amd64 with `-ldflags "-X main.version=vX.Y.Z"`.
   - Produces a `SHA256SUMS.txt` covering every artifact.
   - Creates a draft GitHub Release with the binaries, checksums file, and
     the relevant CHANGELOG section as the release body.
6. A maintainer reviews the draft release and publishes it manually - the
   workflow never auto-publishes, by design, so a human always looks at
   the final artifact list before it goes out.

## Manual local build (what the release workflow automates)

```bash
VERSION=v0.1.0
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  GOOS=${target%/*} GOARCH=${target#*/} \
    go build -ldflags "-X main.version=$VERSION" \
    -o "dist/logquiet-$VERSION-${target%/*}-${target#*/}$( [ "${target%/*}" = windows ] && echo .exe )" \
    ./cmd/logquiet
done
(cd dist && sha256sum * > SHA256SUMS.txt)
```

(`scripts/build-release.sh` in this repository does exactly this; see it
for the exact, current invocation.)

## Package manager metadata

Prepared but not yet published (publishing requires accounts this project
does not control autonomously - see the final report for what remains a
manual, approved step):

- **Homebrew**: a formula template lives at
  `packaging/homebrew/logquiet.rb.tmpl`. Publishing it means creating (or
  getting accepted into) a tap, which requires a GitHub account/organization
  decision - not made automatically by this process.
- **Scoop** (Windows): a manifest template lives at
  `packaging/scoop/logquiet.json.tmpl`. Publishing it means submitting to
  a bucket (or hosting one), same caveat as above.
- **Shell installer**: `scripts/install.sh` downloads the correct release
  asset for the running platform and verifies its checksum against
  `SHA256SUMS.txt` before installing - usable as soon as a GitHub Release
  exists, no additional account needed.

## Verifying a downloaded release

```bash
curl -LO https://github.com/<owner>/logquiet/releases/download/vX.Y.Z/SHA256SUMS.txt
curl -LO https://github.com/<owner>/logquiet/releases/download/vX.Y.Z/logquiet-vX.Y.Z-linux-amd64
sha256sum -c SHA256SUMS.txt --ignore-missing
```

This works because both files land in the same directory (wherever you
ran `curl` from) - `sha256sum -c` resolves the filenames it lists relative
to your *current directory*, not relative to the checksums file's own
location. That distinction matters the moment the two aren't in the same
place: for example, if you build locally with `scripts/build-release.sh`
(which writes into `dist/`) and then try to verify from one level above
with `sha256sum -c dist/SHA256SUMS.txt`, every single entry fails with
"No such file or directory" - not because anything is wrong with the
files, but because `sha256sum` is looking for `./logquiet-...` next to
your shell, not inside `dist/`. This is a real, reproducible gotcha with
`sha256sum -c` itself, not specific to this project, but it is exactly
the kind of thing that makes a release feel broken when it isn't.

To sidestep it entirely, use the wrapper this repository provides, which
works correctly no matter which directory you run it from:

```bash
scripts/verify-release.sh dist                  # a local build directory
scripts/verify-release.sh path/to/SHA256SUMS.txt
```
