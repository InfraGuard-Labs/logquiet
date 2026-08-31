#!/bin/sh
# POSIX sh only (no bashisms) - this is meant to run correctly whether
# invoked as `sh install.sh`, `bash install.sh`, or piped directly into
# `sh` (which ignores the shebang above and just interprets the script
# text, so anything bash-only here would break that invocation silently).
# Downloads the correct LogQuiet release binary for the running
# platform, verifies its checksum against the release's SHA256SUMS.txt,
# and installs it. Works as soon as a GitHub Release exists - no package
# manager account required. macOS and Linux only; Windows users should
# use Scoop or a manual download (see README.md).
#
#   curl -fsSL https://raw.githubusercontent.com/InfraGuard-Labs/logquiet/main/scripts/install.sh | sh
#
# Or download and inspect it first (recommended for anything piped to a
# shell you didn't write):
#   curl -fsSLo install.sh https://raw.githubusercontent.com/InfraGuard-Labs/logquiet/main/scripts/install.sh
#   less install.sh
#   sh install.sh
#
# Environment overrides (mainly for testing against a fixture server or a
# specific release, without editing this file):
#   LOGQUIET_VERSION      Exact tag to install, e.g. v0.1.1 (default: the
#                          latest release, resolved via the GitHub API).
#   LOGQUIET_BASE_URL     Replaces "https://github.com/$REPO/releases/download"
#                          as the release-asset base. The final download URL
#                          is always "$LOGQUIET_BASE_URL/$VERSION/<asset>",
#                          so a mirror or local fixture server must lay
#                          assets out the same way a GitHub Release does.
#   LOGQUIET_INSTALL_DIR  Where the binary is installed (default:
#                          $HOME/.local/bin, which never needs sudo).
#
# This script performs no analytics, telemetry, or any network access
# beyond the two downloads described above (the release API lookup, when
# LOGQUIET_VERSION is unset, and the two file downloads it names on
# stdout before making them).
set -eu

REPO="InfraGuard-Labs/logquiet"
INSTALL_DIR="${LOGQUIET_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${LOGQUIET_VERSION:-latest}"
BASE_URL="${LOGQUIET_BASE_URL:-https://github.com/$REPO/releases/download}"

fail() {
  echo "logquiet install: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "'$1' is required but was not found on PATH. $2"
}

require_cmd curl "Install curl and re-run this script."
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  fail "no checksum tool found (need 'sha256sum' or 'shasum'). Refusing to install without checksum verification."
fi

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Linux) goos="linux" ;;
  Darwin) goos="darwin" ;;
  *) fail "unsupported OS: $os. Download a release manually from https://github.com/$REPO/releases" ;;
esac

case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) fail "unsupported architecture: $arch. Download a release manually from https://github.com/$REPO/releases" ;;
esac

if [ "$VERSION" = "latest" ]; then
  api_url="https://api.github.com/repos/$REPO/releases/latest"
  VERSION="$(curl -fsSL "$api_url" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')" || fail "could not reach $api_url"
  [ -n "$VERSION" ] || fail "could not determine the latest release version from the GitHub API. Set LOGQUIET_VERSION explicitly (e.g. LOGQUIET_VERSION=v0.1.0)."
fi

asset="logquiet-${VERSION}-${goos}-${goarch}"
release_url="${BASE_URL}/${VERSION}"

tmp="$(mktemp -d)" || fail "could not create a temporary directory"
chmod 700 "$tmp"
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT INT TERM

echo "Downloading $asset ($VERSION)..."
if ! curl -fsSL -o "$tmp/$asset" "$release_url/$asset"; then
  fail "download failed: $release_url/$asset (check the version and your network connection; a private/unpublished release or a 404 both look like this)"
fi
if ! curl -fsSL -o "$tmp/SHA256SUMS.txt" "$release_url/SHA256SUMS.txt"; then
  fail "download failed: $release_url/SHA256SUMS.txt"
fi

echo "Verifying checksum..."
if ! grep -q "$asset\$" "$tmp/SHA256SUMS.txt"; then
  fail "no checksum entry for $asset in SHA256SUMS.txt - refusing to install an unverifiable binary"
fi
(
  cd "$tmp"
  if command -v sha256sum >/dev/null 2>&1; then
    grep "$asset\$" SHA256SUMS.txt | sha256sum -c - || fail "checksum verification failed for $asset - the download may be corrupted or tampered with. Nothing was installed."
  else
    expected="$(grep "$asset\$" SHA256SUMS.txt | awk '{print $1}')"
    actual="$(shasum -a 256 "$asset" | awk '{print $1}')"
    [ "$expected" = "$actual" ] || fail "checksum verification failed for $asset (expected $expected, got $actual) - the download may be corrupted or tampered with. Nothing was installed."
  fi
)

if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
  fail "could not create $INSTALL_DIR (permission denied). Either choose a user-writable directory with LOGQUIET_INSTALL_DIR=..., or re-run with sudo: sudo LOGQUIET_INSTALL_DIR=$INSTALL_DIR sh install.sh"
fi
if [ ! -w "$INSTALL_DIR" ]; then
  fail "$INSTALL_DIR is not writable. Either choose a user-writable directory with LOGQUIET_INSTALL_DIR=..., or re-run with sudo: sudo LOGQUIET_INSTALL_DIR=$INSTALL_DIR sh install.sh"
fi

if ! install -m 0755 "$tmp/$asset" "$INSTALL_DIR/logquiet" 2>/dev/null; then
  fail "could not write $INSTALL_DIR/logquiet"
fi

echo "Installed to $INSTALL_DIR/logquiet"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Note: $INSTALL_DIR is not on your PATH. Add it, e.g.: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
"$INSTALL_DIR/logquiet" --version
