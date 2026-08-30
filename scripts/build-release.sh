#!/usr/bin/env bash
# Cross-compiles logquiet for every supported release platform and writes
# a SHA256SUMS.txt covering all artifacts. Requires only the Go toolchain -
# no C cross-compiler, no CGO - which is exactly why Go was chosen for this
# project (see docs/ARCHITECTURE.md).
#
# Usage: scripts/build-release.sh <version>
#   <version> becomes the embedded `logquiet --version` string and part of
#   each artifact's filename. Use a real tag (e.g. v0.1.0) for an actual
#   release, or any label (e.g. "dev") for a local/CI smoke build.
set -euo pipefail

VERSION="${1:?usage: build-release.sh <version>}"
OUT_DIR="dist"
PKG="./cmd/logquiet"

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
)

for target in "${targets[@]}"; do
  os="${target%% *}"
  arch="${target##* }"
  ext=""
  if [ "$os" = "windows" ]; then
    ext=".exe"
  fi
  out="$OUT_DIR/logquiet-${VERSION}-${os}-${arch}${ext}"
  echo "Building $out"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$out" "$PKG"
done

(
  cd "$OUT_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./* > SHA256SUMS.txt
  else
    shasum -a 256 ./* > SHA256SUMS.txt
  fi
)

echo "Done. Artifacts in $OUT_DIR/:"
ls -la "$OUT_DIR"
