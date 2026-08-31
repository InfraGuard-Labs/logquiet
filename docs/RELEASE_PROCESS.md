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

## Recommended next version: v0.1.1, not v0.2.0

The packaging/distribution work (GoReleaser + nfpm automation, `.deb`/
`.rpm` generation, a hardened `install.sh`, Homebrew/Scoop template
refinement) adds no new LogQuiet *behavior* - no new flag, no changed
default, no schema change. Per [Semantic Versioning](https://semver.org/)
and this project's own stated policy above ("minor version bumps (0.x.0)
may include breaking changes to flags or the JSON/impact-report schema"),
a minor bump is reserved for exactly that kind of user-facing change.
Packaging/distribution tooling is not part of the versioned API surface a
user's scripts or flags depend on, so it belongs in a patch release:
**v0.1.1**. Reserve v0.2.0 for the next release that actually changes a
flag, default, or schema.

## Known issue blocking the "latest version" install path

As of this writing, `v0.1.0` is marked as a **pre-release** on GitHub.
GitHub's `/repos/{owner}/{repo}/releases/latest` API endpoint deliberately
excludes pre-releases, which makes `scripts/install.sh`'s default
`LOGQUIET_VERSION=latest` resolution 404 (confirmed by running the script
against the live release). Pinning `LOGQUIET_VERSION=v0.1.0` explicitly
works correctly end-to-end (also confirmed against the live release, in a
disposable container). Un-mark `v0.1.0` as a pre-release (or ensure the
next tagged release is published as a full release, not a pre-release) to
restore the default "latest" path for new users - this is a release-page
setting change, not a code or script change.

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
   tag push and runs [GoReleaser](https://goreleaser.com) against
   [`.goreleaser.yaml`](../.goreleaser.yaml), which:
   - Cross-compiles for linux/amd64, linux/arm64, darwin/amd64,
     darwin/arm64, windows/amd64 with `-trimpath -ldflags "-s -w -X
     main.version=vX.Y.Z"` (same reproducible-build flags
     `scripts/build-release.sh` uses).
   - Packages `.deb` and `.rpm` for linux/amd64 and linux/arm64 via
     [nfpm](https://nfpm.goreleaser.com) (embedded in GoReleaser) - see
     [`packaging/nfpm/nfpm.yaml`](../packaging/nfpm/nfpm.yaml).
   - Produces a `SHA256SUMS.txt` covering every artifact.
   - Generates a Homebrew formula from the same template logic as
     `packaging/homebrew/logquiet.rb.tmpl`, but does **not** push it
     anywhere (`skip_upload: true` is hard-coded in `.goreleaser.yaml`
     until a real tap repository and credentials exist).
   - Creates a draft GitHub Release with the binaries, packages, checksums
     file, and the relevant CHANGELOG section as the release body.
6. A maintainer reviews the draft release and publishes it manually - the
   workflow never auto-publishes, by design, so a human always looks at
   the final artifact list before it goes out.
7. Separately, fill in `packaging/scoop/logquiet.json.tmpl`'s
   `REPLACE_WITH_*` placeholders with the real version and
   `logquiet-vX.Y.Z-windows-amd64.exe` SHA256 from the published release's
   `SHA256SUMS.txt` (GoReleaser does not generate this one - see
   "Package manager metadata" below for why), and publish it to
   `InfraGuard-Labs/scoop-bucket` once that repository exists.

### Local validation before tagging (no publish)

```bash
docker run --rm -v "$PWD":/src -w /src goreleaser/goreleaser:latest \
  release --snapshot --clean --skip=publish
```

Produces real binaries, `.deb`/`.rpm` packages, checksums, and a generated
Homebrew formula under `dist-goreleaser/` (a separate directory from
`dist/`, which holds the actual hand-verified release artifacts - see
[RELEASE_BUILD_RECORD.md](RELEASE_BUILD_RECORD.md) - so a snapshot run
can never overwrite them). Publishes nothing.

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

Prepared but not yet published (publishing requires accounts/repositories
this process does not create autonomously - see below for what remains a
manual, approved step):

- **Homebrew**: `.goreleaser.yaml`'s `brews:` section generates a real
  formula on every release (verified: matches
  `packaging/homebrew/logquiet.rb.tmpl`'s structure, Ruby-syntax-checked).
  Publishing it means creating `InfraGuard-Labs/homebrew-tap` (confirmed,
  as of this writing, not to exist yet - `GET
  api.github.com/repos/InfraGuard-Labs/homebrew-tap` returns 404) and
  removing `skip_upload: true` plus configuring a token with write access
  to that repo - a deliberate, one-time manual step, not automatic.
- **Scoop** (Windows): a manifest template lives at
  `packaging/scoop/logquiet.json.tmpl` (valid JSON, verified). GoReleaser
  does *not* generate this one automatically - its built-in Scoop pipe
  requires a zip-format Windows archive, and this project ships a raw
  `.exe` to match the existing release-asset naming convention (see the
  comment in `.goreleaser.yaml` above the omitted `scoops:` section for
  the full reasoning) - so the manifest is filled in by hand from
  `SHA256SUMS.txt` each release. Publishing means creating
  `InfraGuard-Labs/scoop-bucket` (also confirmed not to exist yet, same
  404 check) and pushing the filled-in manifest there.
- **`.deb` / `.rpm`**: generated by `.goreleaser.yaml`'s `nfpms:` section
  (via [nfpm](https://nfpm.goreleaser.com)) and attached directly to the
  draft GitHub Release - no separate repository or account needed, unlike
  a Homebrew tap or Scoop bucket. Validated (metadata + contents +
  install/run/uninstall) with `dpkg-deb`/`rpm` in disposable containers;
  see `docs/RELEASE_BUILD_RECORD.md`-style evidence for the exact commands
  used, if recorded for a given release.
- **Shell installer**: `scripts/install.sh` downloads the correct release
  asset for the running platform and verifies its checksum against
  `SHA256SUMS.txt` before installing - usable as soon as a GitHub Release
  exists, no additional account needed. Supports `LOGQUIET_VERSION`,
  `LOGQUIET_INSTALL_DIR`, and `LOGQUIET_BASE_URL` overrides; see
  `scripts/tests/test-install.sh` for its behavioral test suite
  (12 scenarios: the 5 supported OS/arch combinations, an unsupported
  architecture, an unsupported OS, a failed download, a 404, a checksum
  mismatch, a non-writable destination, a missing checksum tool, and a
  missing `curl`).

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
