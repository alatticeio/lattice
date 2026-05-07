#!/usr/bin/env bash
# hack/bench/api_bench.sh
#
# Measures control-plane API p99 latency using `hey`.
# Tests GET /api/v1/workspaces (authenticated) under moderate concurrency.
#
# Requires:
#   - hey installed (go install github.com/rakyll/hey@latest)
#   - ADMIN_TOKEN env var with a valid JWT
#   - AGENT_API env var pointing to the latticed HTTP API
#
# Output:
#   Writes hack/bench/results/api-p99.json (Shields.io endpoint format)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
mkdir -p "$RESULTS_DIR"

AGENT_API="${AGENT_API:-http://localhost:8080}"
ADMIN_TOKEN="${ADMIN_TOKEN:?ADMIN_TOKEN is required}"
CONCURRENCY="${BENCH_CONCURRENCY:-10}"
REQUESTS="${BENCH_REQUESTS:-200}"

info() { echo "[api_bench] $*"; }

# ── Install hey if missing ───────────────────────────────────────────────────
if ! command -v hey &>/dev/null; then
  info "Installing hey..."
  go install github.com/rakyll/hey@latest
fi

# ── Run hey ─────────────────────────────────────────────────────────────────
info "Benchmarking GET /api/v1/workspaces (c=${CONCURRENCY}, n=${REQUESTS})..."
HEY_OUTPUT=$(hey \
  -n "$REQUESTS" \
  -c "$CONCURRENCY" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  "${AGENT_API}/api/v1/workspaces" 2>&1)

echo "$HEY_OUTPUT"

# ── Extract p99 (ms) ─────────────────────────────────────────────────────────
# hey output: "  99% in 0.0183 secs"
P99_S=$(echo "$HEY_OUTPUT" | awk '/99%/{print $3}' | head -1)
if [ -z "$P99_S" ]; then
  echo "ERROR: could not parse p99 from hey output" >&2
  exit 1
fi

P99_MS=$(echo "scale=0; $P99_S * 1000 / 1" | bc)
info "p99 latency: ${P99_MS}ms"

# ── SLA check (warn only) ────────────────────────────────────────────────────
SLA_MS=50
if [ "$P99_MS" -gt "$SLA_MS" ]; then
  info "WARN: p99 ${P99_MS}ms exceeds SLA of ${SLA_MS}ms"
fi

# ── Color for badge ──────────────────────────────────────────────────────────
if   [ "$P99_MS" -le 20  ]; then color="brightgreen"
elif [ "$P99_MS" -le 50  ]; then color="green"
elif [ "$P99_MS" -le 100 ]; then color="yellow"
else                              color="red"
fi

# ── Write Shields.io endpoint JSON ───────────────────────────────────────────
cat > "$RESULTS_DIR/api-p99.json" <<EOF
{
  "schemaVersion": 1,
  "label": "API p99",
  "message": "${P99_MS}ms",
  "color": "${color}"
}
EOF

info "Written: $RESULTS_DIR/api-p99.json"
