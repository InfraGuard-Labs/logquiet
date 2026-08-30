#!/usr/bin/env bash
# Captures ONE terminal-mockup screenshot from real LogQuiet output. This is
# the shared machinery behind every demo/run-*.sh script - see demo/README.md
# for the full reproducibility story and demo/run-*.sh for the specific
# commands used for each README screenshot.
#
# Every screenshot produced by this script is a faithful rendering of real
# captured output: the content comes from an actual `logquiet` invocation
# against a checked-in synthetic fixture, piped through demo/ansi2html (a
# literal ANSI-escape-code-to-HTML-span translator, not a reimplementation
# of any rendering decision) and wrapped in a plain terminal-window
# template (demo/gen-screenshot) that only supplies chrome (title bar,
# font, background) - never altering the captured text.
#
# Requires a local Chromium-based browser (Google Chrome or Microsoft
# Edge) for headless screenshotting. No external/hosted service is used.
#
# Usage:
#   demo/capture-screenshot.sh <fixture> <command-label> <title> <out.png> [width] [height]
#
# Example:
#   demo/capture-screenshot.sh demo/fixtures/noisy-app.log \
#     "cat noisy-app.log | logquiet" "logquiet - after" \
#     assets/demo/after.png 760 700
set -euo pipefail

fixture="$1"
command_label="$2"
title="$3"
out_png="$4"
width="${5:-900}"
height="${6:-700}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [ ! -f logquiet.exe ] && [ ! -f logquiet ]; then
  echo "Building logquiet..."
  go build -o logquiet.exe ./cmd/logquiet 2>/dev/null || go build -o logquiet ./cmd/logquiet
fi
LOGQUIET="./logquiet.exe"
[ -f "$LOGQUIET" ] || LOGQUIET="./logquiet"

find_browser() {
  for p in \
    "/c/Program Files/Google/Chrome/Application/chrome.exe" \
    "/c/Program Files (x86)/Google/Chrome/Application/chrome.exe" \
    "/c/Program Files/Microsoft/Edge/Application/msedge.exe" \
    "/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" \
    "/usr/bin/google-chrome" \
    "/usr/bin/chromium-browser" \
    "/usr/bin/chromium" ; do
    if [ -f "$p" ]; then echo "$p"; return 0; fi
  done
  return 1
}

BROWSER="$(find_browser)" || {
  echo "capture-screenshot: no local Chrome/Chromium/Edge found." >&2
  echo "Install one, or open the generated HTML file manually and screenshot it yourself." >&2
  exit 1
}

tmp_html="$(mktemp --suffix=.html 2>/dev/null || echo "/tmp/capture-$$.html")"

"$LOGQUIET" --color "$fixture" \
  | go run ./demo/ansi2html \
  | go run ./demo/gen-screenshot -title "$title" -command "$command_label" -out "$tmp_html"

# Chrome/Edge on Windows need a native Windows path in the file:// URL, not
# Git Bash's POSIX-style /c/... path.
if command -v cygpath >/dev/null 2>&1; then
  win_html="$(cygpath -w "$tmp_html")"
  file_url="file:///${win_html//\\//}"
else
  file_url="file://$tmp_html"
fi

mkdir -p "$(dirname "$out_png")"
# Chrome resolves a relative -screenshot path against its own idea of the
# current directory, which is not reliably the shell's CWD once launched
# as a native Windows process from Git Bash - pass an absolute, native
# path to avoid "cannot find the path specified" failures.
if command -v cygpath >/dev/null 2>&1; then
  out_native="$(cygpath -w "$(cd "$(dirname "$out_png")" && pwd)/$(basename "$out_png")")"
else
  out_native="$(cd "$(dirname "$out_png")" && pwd)/$(basename "$out_png")"
fi
"$BROWSER" --headless --disable-gpu --screenshot="$out_native" --window-size="${width},${height}" "$file_url"
rm -f "$tmp_html"

echo "Wrote $out_png"
