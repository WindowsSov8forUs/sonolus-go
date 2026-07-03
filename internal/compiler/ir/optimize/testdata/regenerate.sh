#!/usr/bin/env bash
# Regenerates optimize_golden.json from sonolus.py's real optimizer passes.
# Requires a local sonolus.py checkout (Python 3.13+).
set -euo pipefail
SONOLUS_PY="${1:-/d/Documents/VSCode/sonolus.py}"
HERE="$(cd "$(dirname "$0")" && pwd)"
PYTHONPATH="$SONOLUS_PY" python "$HERE/harness.py" > "$HERE/optimize_golden.json"
echo "wrote $HERE/optimize_golden.json"
