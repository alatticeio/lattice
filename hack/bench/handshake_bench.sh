#!/usr/bin/env bash
# hack/bench/handshake_bench.sh
#
# Measures Time-to-First-Handshake (TTFH) for the LAN connection scenario.
#
# Requires:
#   - Docker with lattice-k3s image running (or started by bench.yml)
#   - AGENT_IMAGE env var pointing to the lattice agent image
#   - TOKEN env var with a valid enrollment token
#   - AGENT_API env var pointing to the latticed HTTP API
#
# Output:
#   Writes hack/bench/results/handshake.json (Shields.io endpoint format)
#
# Usage:
#   TOKEN=xxx AGENT_API=http://localhost:8080 AGENT_IMAGE=ghcr.io/alatticeio/lattice:latest \
#     ./hack/bench/handshake_bench.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
mkdir -p "$RESULTS_DIR"

AGENT_IMAGE="${AGENT_IMAGE:-ghcr.io/alatticeio/lattice:latest}"
AGENT_API="${AGENT_API:-http://localhost:8080}"
TOKEN="${TOKEN:?TOKEN is required}"
RUNS="${RUNS:-3}"

info() { echo "[handshake_bench] $*"; }

cleanup_containers() {
  docker rm -f bench-agent-a bench-agent-b 2>/dev/null || true
  docker network rm bench-net 2>/dev/null || true
}

# ── Run benchmark RUNS times, collect samples ──────────────────────────────
samples=()

for i in $(seq 1 "$RUNS"); do
  info "Run $i/$RUNS"
  SUFFIX=$(shuf -i 10000-99999 -n 1)
  cleanup_containers

  docker network create bench-net >/dev/null

  # t0: record before starting agent
  T0=$(date +%s%3N)  # milliseconds

  docker run -d \
    --name bench-agent-a \
    --privileged \
    --network bench-net \
    --add-host=host.docker.internal:host-gateway \
    -e LATTICE_APP_ID="bench-a-${SUFFIX}" \
    "$AGENT_IMAGE" \
    up --server-url "$AGENT_API" --token "$TOKEN" --level warn

  docker run -d \
    --name bench-agent-b \
    --privileged \
    --network bench-net \
    --add-host=host.docker.internal:host-gateway \
    -e LATTICE_APP_ID="bench-b-${SUFFIX}" \
    "$AGENT_IMAGE" \
    up --server-url "$AGENT_API" --token "$TOKEN" --level warn

  # Poll agent-a until wg show reports a latest handshake (t1)
  info "  Waiting for first WireGuard handshake..."
  DEADLINE=$(( $(date +%s) + 30 ))
  HANDSHAKE_MS=""
  while [ "$(date +%s)" -lt "$DEADLINE" ]; do
    if docker exec bench-agent-a wg show wf0 2>/dev/null \
        | grep -q "latest handshake"; then
      T1=$(date +%s%3N)
      HANDSHAKE_MS=$(( T1 - T0 ))
      break
    fi
    sleep 0.5
  done

  if [ -z "$HANDSHAKE_MS" ]; then
    info "  TIMEOUT: no handshake within 30s — skipping run"
    continue
  fi

  info "  TTFH = ${HANDSHAKE_MS}ms"
  samples+=("$HANDSHAKE_MS")
done

# ── Compute average ─────────────────────────────────────────────────────────
if [ "${#samples[@]}" -eq 0 ]; then
  echo "ERROR: all runs timed out" >&2
  exit 1
fi

total=0
for s in "${samples[@]}"; do total=$(( total + s )); done
avg_ms=$(( total / ${#samples[@]} ))
avg_s=$(echo "scale=2; $avg_ms / 1000" | bc)

info "Average TTFH over ${#samples[@]} runs: ${avg_s}s"

# ── SLA check (warn only — gate deferred) ───────────────────────────────────
SLA_MS=3000
if [ "$avg_ms" -gt "$SLA_MS" ]; then
  info "WARN: avg TTFH ${avg_s}s exceeds LAN SLA of 3s"
fi

# ── Color for badge ─────────────────────────────────────────────────────────
if   [ "$avg_ms" -le 2000 ]; then color="brightgreen"
elif [ "$avg_ms" -le 3000 ]; then color="green"
elif [ "$avg_ms" -le 5000 ]; then color="yellow"
else                               color="red"
fi

# ── Write Shields.io endpoint JSON ──────────────────────────────────────────
cat > "$RESULTS_DIR/handshake.json" <<EOF
{
  "schemaVersion": 1,
  "label": "handshake (LAN)",
  "message": "${avg_s}s",
  "color": "${color}"
}
EOF

info "Written: $RESULTS_DIR/handshake.json"
