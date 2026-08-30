#!/usr/bin/env bash
# Reproduces assets/demo/frequency-spike.png: a brand-new error class with
# zero prior history suddenly firing at high volume, demonstrating the
# deterministic, non-AI frequency-spike (bootstrap) anomaly detector - see
# docs/TECHNICAL_METHOD.md section 7. This scenario is deterministic even
# though anomaly detection is wall-clock based, because reading a static
# file happens fast enough that the bootstrap path (which needs enough
# *events*, not real elapsed time) fires reliably every run.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
exec bash demo/capture-screenshot.sh \
  demo/fixtures/frequency-spike.log \
  "cat frequency-spike.log | logquiet" \
  "logquiet - frequency spike" \
  assets/demo/frequency-spike.png \
  800 680
