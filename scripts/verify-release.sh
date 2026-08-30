#!/usr/bin/env bash
# Verifies release artifacts against SHA256SUMS.txt, regardless of the
# caller's current directory.
#
# This exists because `sha256sum -c` resolves the filenames listed inside
# a checksums file relative to the CALLER'S current working directory, not
# relative to the checksums file's own location - so a perfectly natural
# command like `sha256sum -c dist/SHA256SUMS.txt`, run from one level
# above dist/, fails with "No such file or directory" for every single
# entry even though nothing is actually wrong with the downloaded files.
# This script does the right thing (cd into the checksums file's own
# directory before checking) so users do not have to know that.
#
# Usage:
#   scripts/verify-release.sh path/to/SHA256SUMS.txt
#   scripts/verify-release.sh path/to/dist          # or just the directory
#   scripts/verify-release.sh                       # defaults to ./SHA256SUMS.txt
set -euo pipefail

target="${1:-SHA256SUMS.txt}"
if [ -d "$target" ]; then
  target="$target/SHA256SUMS.txt"
fi

if [ ! -f "$target" ]; then
  echo "verify-release: checksums file not found: $target" >&2
  exit 1
fi

dir="$(cd "$(dirname "$target")" && pwd)"
file="$(basename "$target")"

echo "Verifying artifacts listed in $dir/$file ..."
(
  cd "$dir"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c "$file"
  else
    shasum -a 256 -c "$file"
  fi
)
