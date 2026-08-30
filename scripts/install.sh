#!/usr/bin/env bash
# Downloads the correct LogQuiet release binary for the running
# platform, verifies its checksum against the release's SHA256SUMS.txt,
# and installs it. Works as soon as a GitHub Release exists - no package
# manager account required.
#
#   curl -fsSL https://raw.githubusercontent.com/azimsiddiqui/logquiet/main/scripts/install.sh | bash
#
# Or download and inspect it first (recommended for anything piped to a
# shell you didn't write):
#   curl -fsSLo install.sh https://raw.githubusercontent.com/azimsiddiqui/logquiet/main/scripts/install.sh
#   less install.sh
#   bash install.sh
set -euo pipefail

REPO="azimsiddiqui/logquiet"
INSTALL_DIR="${LOGQUIET_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${LOGQUIET_VERSION:-latest}"

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Linux) goos="linux" ;;
  Darwin) goos="darwin" ;;
  *) echo "Unsupported OS: $os. Download a release manually from https://github.com/$REPO/releases" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) echo "Unsupported architecture: $arch. Download a release manually from https://github.com/$REPO/releases" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  api_url="https://api.github.com/repos/$REPO/releases/latest"
  VERSION="$(curl -fsSL "$api_url" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  if [ -z "$VERSION" ]; then
    echo "Could not determine the latest release version. Set LOGQUIET_VERSION explicitly." >&2
    exit 1
  fi
fi

asset="logquiet-${VERSION}-${goos}-${goarch}"
base_url="https://github.com/$REPO/releases/download/${VERSION}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $asset ($VERSION)..."
curl -fsSL -o "$tmp/$asset" "$base_url/$asset"
curl -fsSL -o "$tmp/SHA256SUMS.txt" "$base_url/SHA256SUMS.txt"

echo "Verifying checksum..."
(
  cd "$tmp"
  if command -v sha256sum >/dev/null 2>&1; then
    grep "$asset\$" SHA256SUMS.txt | sha256sum -c -
  else
    expected="$(grep "$asset\$" SHA256SUMS.txt | awk '{print $1}')"
    actual="$(shasum -a 256 "$asset" | awk '{print $1}')"
    [ "$expected" = "$actual" ] || { echo "Checksum mismatch!" >&2; exit 1; }
  fi
)

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$asset" "$INSTALL_DIR/logquiet"

echo "Installed to $INSTALL_DIR/logquiet"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Note: $INSTALL_DIR is not on your PATH. Add it, e.g.: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
"$INSTALL_DIR/logquiet" --version
