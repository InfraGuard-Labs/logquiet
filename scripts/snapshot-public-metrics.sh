#!/usr/bin/env bash
# Pulls public GitHub metrics for InfraGuard-Labs/logquiet (stars, forks,
# open issues, per-release asset download counts) via GitHub's public
# REST API, prints them, and writes a timestamped JSON snapshot.
#
# No credentials are required for basic usage - all endpoints used here
# are public. An optional GITHUB_TOKEN environment variable is used, if
# set, only to raise the API rate limit (60 req/hour unauthenticated vs.
# 5000 req/hour authenticated); it is never required.
#
# This script does NOT and cannot access private traffic metrics (unique
# visitors/clones from the repository's "Traffic" insights page) - those
# require authenticated owner access via the GitHub UI or the
# /traffic/* API endpoints AND have a short (14-day) retention window
# that GitHub does not extend. See "Traffic metrics" below for the
# manual preservation process this project uses instead.
#
# Usage:
#   scripts/snapshot-public-metrics.sh [output-dir]
#   GITHUB_TOKEN=ghp_xxx scripts/snapshot-public-metrics.sh
#
# Output: <output-dir>/metrics-snapshot-<UTC timestamp>.json (default
# output-dir: evidence/adoption/snapshots)
set -euo pipefail

REPO="InfraGuard-Labs/logquiet"
API="https://api.github.com/repos/$REPO"
OUT_DIR="${1:-evidence/adoption/snapshots}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_FILE="$OUT_DIR/metrics-snapshot-${TIMESTAMP}.json"

command -v curl >/dev/null 2>&1 || { echo "snapshot-public-metrics: curl is required" >&2; exit 1; }

AUTH_HEADER=()
if [ -n "${GITHUB_TOKEN:-}" ]; then
  AUTH_HEADER=(-H "Authorization: Bearer $GITHUB_TOKEN")
fi

api_get() {
  curl -fsSL "${AUTH_HEADER[@]}" -H "Accept: application/vnd.github+json" "$1"
}

echo "Fetching public repository metadata for $REPO..."
repo_json="$(api_get "$API")"

# Minimal, dependency-free JSON field extraction (no jq requirement) -
# these fields are simple top-level integers/strings in GitHub's
# response, so a targeted grep/sed is reliable here without pulling in a
# JSON parser dependency for a small local script.
extract_number() {
  printf '%s' "$1" | grep -o "\"$2\":[[:space:]]*[0-9]*" | head -1 | sed -E "s/\"$2\":[[:space:]]*//"
}

stars="$(extract_number "$repo_json" "stargazers_count")"
forks="$(extract_number "$repo_json" "forks_count")"
open_issues="$(extract_number "$repo_json" "open_issues_count")"

echo "Fetching releases and per-asset download counts..."
releases_json="$(api_get "$API/releases")"

mkdir -p "$OUT_DIR"

{
  echo "{"
  echo "  \"snapshot_generated_at_utc\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
  echo "  \"repository\": \"$REPO\","
  echo "  \"github_stars\": ${stars:-null},"
  echo "  \"github_forks\": ${forks:-null},"
  echo "  \"github_open_issues_count\": ${open_issues:-null},"
  echo "  \"note_on_open_issues_count\": \"GitHub's open_issues_count includes open pull requests; it is NOT the same as external_issues in docs/METRICS_DEFINITIONS.md, which requires filtering by author.\","
  echo "  \"releases\": ["
  # Re-parse releases_json into per-release, per-asset download counts.
  # Deliberately simple line-oriented extraction, not a hand-rolled JSON
  # parser - this uses whichever of python3/python/node is available, in
  # that order, and degrades honestly (not silently) if none are.
  PY_BIN=""
  for candidate in python3 python; do
    command -v "$candidate" >/dev/null 2>&1 && { PY_BIN="$candidate"; break; }
  done
  if [ -n "$PY_BIN" ]; then
    printf '%s' "$releases_json" | "$PY_BIN" -c '
import json, sys
data = json.load(sys.stdin)
out = []
for rel in data:
    assets = [{"name": a["name"], "download_count": a["download_count"]} for a in rel.get("assets", [])]
    out.append({"tag_name": rel["tag_name"], "draft": rel["draft"], "assets": assets})
print(json.dumps(out, indent=2))
' | sed 's/^/    /'
  elif command -v node >/dev/null 2>&1; then
    printf '%s' "$releases_json" | node -e '
let input = "";
process.stdin.on("data", d => input += d);
process.stdin.on("end", () => {
  const data = JSON.parse(input);
  const out = data.map(rel => ({
    tag_name: rel.tag_name,
    draft: rel.draft,
    assets: (rel.assets || []).map(a => ({ name: a.name, download_count: a.download_count })),
  }));
  console.log(JSON.stringify(out, null, 2));
});
' | sed 's/^/    /'
  else
    echo "    \"raw_extraction_unavailable_no_python_or_node_found\": true"
  fi
  echo "  ]"
  echo "}"
} > "$OUT_FILE"

echo
echo "Wrote $OUT_FILE"
echo
echo "== Summary =="
echo "stars:        ${stars:-unknown}"
echo "forks:        ${forks:-unknown}"
echo "open issues:  ${open_issues:-unknown} (includes open PRs - see note in JSON output)"
echo
echo "Per-release download counts are in $OUT_FILE (structured extraction needs python3, python, or node; raw API response is otherwise available by re-running: curl $API/releases)."
echo
echo "API limitations documented in this script's header: unauthenticated"
echo "requests are capped at 60/hour; set GITHUB_TOKEN to raise that to"
echo "5000/hour. Nothing here reflects unique users - see"
echo "docs/METRICS_DEFINITIONS.md \"downloads != unique users\"."
echo
echo "This script cannot fetch private traffic data (unique visitors/"
echo "clones). See docs/METRICS_DEFINITIONS.md and evidence/README.md"
echo "\"Traffic metrics\" for the manual monthly process for that data."
