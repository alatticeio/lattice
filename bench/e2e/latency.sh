#!/usr/bin/env bash
# bench/e2e/latency.sh
# Measures overlay vs direct ping RTT.
# Usage: ./latency.sh <direct_ip> <overlay_ip> [count]

set -euo pipefail

DIRECT_IP="${1:?Usage: $0 <direct_ip> <overlay_ip> [count]}"
OVERLAY_IP="${2:?Usage: $0 <direct_ip> <overlay_ip> [count]}"
COUNT="${3:-100}"
OUTPUT_DIR="$(dirname "$0")/../results"
TIMESTAMP=$(date +%Y%m%dT%H%M%S)

mkdir -p "$OUTPUT_DIR"

extract_avg_rtt() {
  # Extracts avg RTT in ms from ping output (works on Linux and macOS).
  grep -E 'rtt|round-trip' | grep -oE '[0-9]+\.[0-9]+/[0-9]+\.[0-9]+' | cut -d/ -f2
}

echo "=== Direct ping ($DIRECT_IP, $COUNT packets) ==="
DIRECT_OUT=$(ping -c "$COUNT" -q "$DIRECT_IP" 2>&1)
DIRECT_RTT=$(echo "$DIRECT_OUT" | extract_avg_rtt)
echo "$DIRECT_OUT"

echo ""
echo "=== Overlay ping ($OVERLAY_IP, $COUNT packets) ==="
OVERLAY_OUT=$(ping -c "$COUNT" -q "$OVERLAY_IP" 2>&1)
OVERLAY_RTT=$(echo "$OVERLAY_OUT" | extract_avg_rtt)
echo "$OVERLAY_OUT"

DELTA=$(echo "scale=2; $OVERLAY_RTT - $DIRECT_RTT" | bc)

echo ""
echo "| Scenario | Direct | Overlay | Delta |"
echo "|----------|--------|---------|-------|"
echo "| ping RTT | ${DIRECT_RTT} ms | ${OVERLAY_RTT} ms | +${DELTA} ms |"

jq -n \
  --argjson dr "$DIRECT_RTT" --argjson or_ "$OVERLAY_RTT" \
  '{timestamp: "'"$TIMESTAMP"'", direct_rtt_ms: $dr, overlay_rtt_ms: $or_}' \
  > "$OUTPUT_DIR/latency_${TIMESTAMP}.json"
