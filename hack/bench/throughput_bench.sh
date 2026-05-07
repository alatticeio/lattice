#!/usr/bin/env bash
# hack/bench/throughput_bench.sh
#
# Measures WireGuard tunnel throughput using iperf3.
# Runs iperf3 server on bench-agent-b, client on bench-agent-a,
# traffic flows through the WireGuard overlay (wf0 interface).
#
# Prerequisites:
#   - bench-agent-a and bench-agent-b already running and connected
#     (call handshake_bench.sh first, or ensure containers are up)
#   - Both agent images must include iperf3
#
# Output:
#   Writes hack/bench/results/throughput.json (Shields.io endpoint format)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
mkdir -p "$RESULTS_DIR"

DURATION="${IPERF_DURATION:-10}"  # seconds per run

info() { echo "[throughput_bench] $*"; }

# ── Get Agent B's WireGuard IP ──────────────────────────────────────────────
info "Resolving Agent B WireGuard IP..."
WG_IP_B=$(docker exec bench-agent-b \
  ip -4 addr show wf0 2>/dev/null \
  | awk '/inet /{print $2}' | cut -d'/' -f1 || true)

if [ -z "$WG_IP_B" ]; then
  echo "ERROR: could not determine Agent B WireGuard IP. Is wf0 up?" >&2
  exit 1
fi
info "Agent B WireGuard IP: $WG_IP_B"

# ── Start iperf3 server on Agent B ──────────────────────────────────────────
docker exec -d bench-agent-b iperf3 -s -D 2>/dev/null || true
sleep 1

# ── Run iperf3 from Agent A to Agent B over WireGuard ───────────────────────
info "Running iperf3 for ${DURATION}s..."
IPERF_OUTPUT=$(docker exec bench-agent-a \
  iperf3 -c "$WG_IP_B" -t "$DURATION" -J 2>/dev/null || echo "{}")

# Extract Mbps from JSON output (sender bitrate)
MBPS=$(echo "$IPERF_OUTPUT" \
  | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    bps = d['end']['sum_sent']['bits_per_second']
    print(f'{bps/1e6:.0f}')
except Exception:
    print('0')
" 2>/dev/null || echo "0")

info "Throughput: ${MBPS} Mbps"

# ── SLA check (warn only) ────────────────────────────────────────────────────
SLA_MBPS=800
if [ "${MBPS:-0}" -lt "$SLA_MBPS" ] 2>/dev/null; then
  info "WARN: throughput ${MBPS} Mbps below SLA of ${SLA_MBPS} Mbps"
fi

# ── Color for badge ──────────────────────────────────────────────────────────
MBPS_INT="${MBPS:-0}"
if   [ "$MBPS_INT" -ge 900 ]; then color="brightgreen"
elif [ "$MBPS_INT" -ge 800 ]; then color="green"
elif [ "$MBPS_INT" -ge 600 ]; then color="yellow"
else                               color="red"
fi

# ── Write Shields.io endpoint JSON ───────────────────────────────────────────
cat > "$RESULTS_DIR/throughput.json" <<EOF
{
  "schemaVersion": 1,
  "label": "throughput",
  "message": "${MBPS} Mbps",
  "color": "${color}"
}
EOF

info "Written: $RESULTS_DIR/throughput.json"
