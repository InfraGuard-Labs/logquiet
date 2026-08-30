#!/usr/bin/env bash
# Reproduces assets/demo/before.png: the raw, unprocessed noisy-app.log
# fixture as a developer would actually see it with `cat`/`tail` - no
# LogQuiet involved, to show what the problem looks like before it exists.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

tmp_html="$(mktemp --suffix=.html 2>/dev/null || echo "/tmp/before-$$.html")"
cat demo/fixtures/noisy-app.log \
  | go run ./demo/ansi2html \
  | go run ./demo/gen-screenshot -title "noisy-app.log - raw" -command "cat noisy-app.log" -out "$tmp_html"

if command -v cygpath >/dev/null 2>&1; then
  win_html="$(cygpath -w "$tmp_html")"
  file_url="file:///${win_html//\\//}"
else
  file_url="file://$tmp_html"
fi

BROWSER=""
for p in \
  "/c/Program Files/Google/Chrome/Application/chrome.exe" \
  "/c/Program Files (x86)/Google/Chrome/Application/chrome.exe" \
  "/c/Program Files/Microsoft/Edge/Application/msedge.exe" \
  "/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" \
  "/usr/bin/google-chrome" "/usr/bin/chromium-browser" "/usr/bin/chromium"; do
  [ -f "$p" ] && BROWSER="$p" && break
done
[ -n "$BROWSER" ] || { echo "no local Chrome/Chromium/Edge found" >&2; exit 1; }

mkdir -p assets/demo
out_native="$(cygpath -w "$(pwd)/assets/demo/before.png" 2>/dev/null || echo "$(pwd)/assets/demo/before.png")"
"$BROWSER" --headless --disable-gpu --screenshot="$out_native" --window-size=900,1300 "$file_url"
rm -f "$tmp_html"
echo "Wrote assets/demo/before.png"
