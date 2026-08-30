#!/usr/bin/env bash
# Reproduces assets/demo/after.png: the same noisy-app.log fixture as
# run-before.sh, this time processed through logquiet - showing the
# routine noise collapsed and the critical failure/traceback retained.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
exec bash demo/capture-screenshot.sh \
  demo/fixtures/noisy-app.log \
  "cat noisy-app.log | logquiet" \
  "logquiet - after" \
  assets/demo/after.png \
  760 700
