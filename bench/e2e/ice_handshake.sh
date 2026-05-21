#!/usr/bin/env bash
# bench/e2e/ice_handshake.sh
# Extracts ICE handshake timing from lattice agent logs.
# Usage: ./ice_handshake.sh <log_file>
#
# The lattice agent logs "ICE connected" with timing when a peer connects.
# Run this after initiating a fresh peer connection with lattice logs captured.

set -euo pipefail

LOG_FILE="${1:?Usage: $0 <lattice_log_file>}"

echo "=== ICE Handshake Timing from $LOG_FILE ==="
echo ""

# Extract SYN and Connected timestamps for each peer.
# Log format example:
#   2026-05-21T10:00:00Z INFO starting ICE dial peer=peer-a
#   2026-05-21T10:00:02Z INFO ICE connected peer=peer-a elapsed=2.1s
grep -E "starting ICE dial|ICE connected" "$LOG_FILE" | \
  awk '
    /starting ICE dial/ {
      match($0, /peer=([^ ]+)/, arr)
      peer = arr[1]
      match($0, /^([^ ]+)/, ts)
      start[peer] = ts[1]
    }
    /ICE connected/ {
      match($0, /peer=([^ ]+)/, arr)
      peer = arr[1]
      match($0, /elapsed=([^ ]+)/, el)
      print peer, "SYN→Connected:", el[1]
    }
  '

echo ""
echo "Tip: block UDP with 'iptables -I INPUT -p udp --dport 51820 -j DROP' to test LRP relay fallback."
