#!/usr/bin/env bash
# Reproduces assets/demo/stack-trace.png: routine worker chatter with one
# full Python traceback buried in the middle, demonstrating that multiline
# exceptions are preserved in full while the surrounding routine lines are
# collapsed - see docs/TECHNICAL_METHOD.md section 1.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
exec bash demo/capture-screenshot.sh \
  demo/fixtures/stack-trace.log \
  "cat stack-trace.log | logquiet" \
  "logquiet - stack trace preserved" \
  assets/demo/stack-trace.png \
  760 420
