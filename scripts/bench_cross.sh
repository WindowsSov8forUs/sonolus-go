#!/bin/bash
# Cross-project performance baseline: sonolus-go vs sonolus.py
#
# Measures compilation throughput for equivalent engine definitions.
# Requires:
#   - sonolus-go built (go build ./cmd/sonolus-go/)
#   - sonolus.py at ../sonolus.py with Python >= 3.11
#
# Usage: bash scripts/bench_cross.sh [iterations]

set -euo pipefail

N="${1:-50}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJ_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== sonolus-go vs sonolus.py performance baseline ==="
echo "iterations: $N"
echo ""

# ── Go benchmark ──
echo "--- sonolus-go (compiler/engine benchmarks) ---"
cd "$PROJ_DIR"
go test ./compiler/engine/ -bench . -benchtime="${N}x" -benchmem 2>&1 | grep -E "Benchmark|ns/op|allocs/op"
echo ""

# ── Python benchmark (if available) ──
PY_DIR="../sonolus.py"
if [ -d "$PY_DIR" ]; then
    echo "--- sonolus.py (compile benchmark) ---"
    cd "$PY_DIR"
    # Time compilation of the pydori test engine.
    if [ -f "test_projects/pydori/engine.py" ]; then
        echo "Compiling pydori engine ($N iterations)..."
        START=$(date +%s%N)
        for i in $(seq 1 "$N"); do
            python -m sonolus.build.cli build test_projects/pydori/engine.py -o /tmp/sonolus-py-bench >/dev/null 2>&1 || true
        done
        END=$(date +%s%N)
        ELAPSED_MS=$(( (END - START) / 1000000 ))
        AVG_MS=$(( ELAPSED_MS / N ))
        echo "  total: ${ELAPSED_MS}ms, avg: ${AVG_MS}ms/compile"
    else
        echo "  pydori test engine not found, skipping Python benchmark"
    fi
else
    echo "--- sonolus.py ---"
    echo "  sonolus.py not found at $PY_DIR, skipping"
fi

echo ""
echo "=== Done ==="
