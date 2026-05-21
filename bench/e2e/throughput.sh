#!/usr/bin/env bash
# bench/e2e/throughput.sh
# Measures overlay vs bare-metal TCP/UDP throughput using iperf3.
# Usage: ./throughput.sh <server_ip> <overlay_server_ip> [duration_sec]
#
# Prerequisites: iperf3 installed on both machines, lattice tunnel up.
# Run iperf3 server on remote: iperf3 -s -D

set -euo pipefail

SERVER_IP="${1:?Usage: $0 <bare_ip> <overlay_ip> [duration]}"
OVERLAY_IP="${2:?Usage: $0 <bare_ip> <overlay_ip> [duration]}"
DURATION="${3:-10}"
OUTPUT_DIR="$(dirname "$0")/../results"
TIMESTAMP=$(date +%Y%m%dT%H%M%S)

mkdir -p "$OUTPUT_DIR"

echo "=== TCP Bare Metal ==="
iperf3 -c "$SERVER_IP" -t "$DURATION" -J > "$OUTPUT_DIR/tcp_bare_${TIMESTAMP}.json"
TCP_BARE=$(jq '.end.sum_received.bits_per_second / 1e6 | round' "$OUTPUT_DIR/tcp_bare_${TIMESTAMP}.json")

echo "=== TCP Overlay ==="
iperf3 -c "$OVERLAY_IP" -t "$DURATION" -J > "$OUTPUT_DIR/tcp_overlay_${TIMESTAMP}.json"
TCP_OVERLAY=$(jq '.end.sum_received.bits_per_second / 1e6 | round' "$OUTPUT_DIR/tcp_overlay_${TIMESTAMP}.json")

echo "=== UDP Bare Metal ==="
iperf3 -c "$SERVER_IP" -u -b 1G -t "$DURATION" -J > "$OUTPUT_DIR/udp_bare_${TIMESTAMP}.json"
UDP_BARE=$(jq '.end.sum.bits_per_second / 1e6 | round' "$OUTPUT_DIR/udp_bare_${TIMESTAMP}.json")

echo "=== UDP Overlay ==="
iperf3 -c "$OVERLAY_IP" -u -b 1G -t "$DURATION" -J > "$OUTPUT_DIR/udp_overlay_${TIMESTAMP}.json"
UDP_OVERLAY=$(jq '.end.sum.bits_per_second / 1e6 | round' "$OUTPUT_DIR/udp_overlay_${TIMESTAMP}.json")

TCP_OVERHEAD=$(echo "scale=1; (1 - $TCP_OVERLAY / $TCP_BARE) * 100" | bc)
UDP_OVERHEAD=$(echo "scale=1; (1 - $UDP_OVERLAY / $UDP_BARE) * 100" | bc)

echo ""
echo "| Scenario | Bare Metal | Overlay | Overhead |"
echo "|----------|-----------|---------|----------|"
echo "| TCP      | ${TCP_BARE} Mbps | ${TCP_OVERLAY} Mbps | ${TCP_OVERHEAD}% |"
echo "| UDP      | ${UDP_BARE} Mbps | ${UDP_OVERLAY} Mbps | ${UDP_OVERHEAD}% |"

# Save summary JSON for plot.py
jq -n \
  --argjson tb "$TCP_BARE" --argjson to "$TCP_OVERLAY" \
  --argjson ub "$UDP_BARE" --argjson uo "$UDP_OVERLAY" \
  '{timestamp: "'"$TIMESTAMP"'", tcp_bare_mbps: $tb, tcp_overlay_mbps: $to, udp_bare_mbps: $ub, udp_overlay_mbps: $uo}' \
  > "$OUTPUT_DIR/summary_${TIMESTAMP}.json"

echo ""
echo "Results saved to $OUTPUT_DIR/"
