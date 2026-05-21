#!/usr/bin/env bash
# bench/scripts/run_all.sh
# Runs all Go benchmarks and saves results to bench/results/.
# Usage: ./run_all.sh [count]

set -euo pipefail

COUNT="${1:-5}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RESULTS_DIR="$REPO_ROOT/bench/results"
TIMESTAMP=$(date +%Y%m%dT%H%M%S)

mkdir -p "$RESULTS_DIR"

echo "=== Component benchmarks (bench/go/) ==="
go test -bench=. -benchmem -count="$COUNT" "$REPO_ROOT/bench/go/..." \
  | tee "$RESULTS_DIR/component_${TIMESTAMP}.txt"

echo ""
echo "=== Integration benchmarks (bench/docker/) ==="
go test -bench=. -benchmem -count="$COUNT" -timeout=300s "$REPO_ROOT/bench/docker/..." \
  | tee "$RESULTS_DIR/integration_${TIMESTAMP}.txt"

echo ""
echo "=== benchstat summary ==="
if command -v benchstat &>/dev/null; then
  benchstat "$RESULTS_DIR/component_${TIMESTAMP}.txt"
else
  echo "benchstat not found. Install with: go install golang.org/x/perf/cmd/benchstat@latest"
fi

echo ""
echo "Results saved to $RESULTS_DIR/"
