#!/usr/bin/env bash
# Regenerates every deterministic demo fixture under demo/fixtures/ (used
# by run-before.sh, run-after.sh, run-frequency-spike.sh, and
# run-stack-trace.sh). Safe to re-run any time - output is byte-identical
# on every run since nothing here depends on the real clock or randomness;
# see tools/fixturegen/demo.go.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
go run ./tools/fixturegen demo
echo "Wrote demo/fixtures/*.log"
