#!/usr/bin/env sh
# Lattice Sandbox Entrypoint
#
# Reads configuration from environment variables and starts `lattice sandbox start`.
# This script runs inside the sandbox container — users do not invoke it directly.
#
# Required env vars:
#   LATTICE_SERVER_URL  — Control plane URL
#   LATTICE_TOKEN       — Enrollment token (one-time use for first registration)
#   LATTICE_NAME        — Sandbox identifier
#
# Optional env vars:
#   PROXY_ADDR          — SOCKS5 listen address (default 127.0.0.1:1080, set "" to disable)
#   FORWARD_RULES       — Comma-separated inbound forward rules (e.g. "8080:127.0.0.1:8080")
#   EGRESS_ALLOW        — Comma-separated allowed egress CIDRs
#   EGRESS_DEFAULT_DENY — Set "true" to block all non-white-listed egress
#   WG_PORT             — WireGuard port (default 51820)
#   LOG_LEVEL           — Log verbosity (default info)

set -e

# ── required ─────────────────────────────────────────────────────────────────
if [ -z "$LATTICE_SERVER_URL" ] || [ -z "$LATTICE_TOKEN" ] || [ -z "$LATTICE_NAME" ]; then
  echo "ERROR: LATTICE_SERVER_URL, LATTICE_TOKEN, and LATTICE_NAME are required"
  echo "Example:"
  echo "  docker run -e LATTICE_SERVER_URL=https://lattice.example.com \\"
  echo "             -e LATTICE_TOKEN=lt-xxxxxxxx \\"
  echo "             -e LATTICE_NAME=ai-agent-01 \\"
  echo "             lattice-sandbox"
  exit 1
fi

# ── build args ───────────────────────────────────────────────────────────────
set -- \
  "sandbox" "start" \
  "--name" "$LATTICE_NAME" \
  "--server-url" "$LATTICE_SERVER_URL" \
  "--token" "$LATTICE_TOKEN"

# SOCKS5 proxy (default on)
if [ "${PROXY_ADDR}" != "" ]; then
  set -- "$@" "--proxy-addr" "${PROXY_ADDR:-127.0.0.1:1080}"
fi

# Forward rules
if [ -n "$FORWARD_RULES" ]; then
  OLD_IFS="$IFS"
  IFS=","
  for rule in $FORWARD_RULES; do
    rule="$(echo "$rule" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    if [ -n "$rule" ]; then
      set -- "$@" "--forward" "$rule"
    fi
  done
  IFS="$OLD_IFS"
fi

# Egress allow
if [ -n "$EGRESS_ALLOW" ]; then
  set -- "$@" "--egress-allow" "$EGRESS_ALLOW"
fi

# Egress default deny
if [ "${EGRESS_DEFAULT_DENY}" = "true" ]; then
  set -- "$@" "--egress-default-deny"
fi

echo "Starting lattice sandbox: $@"
exec /app/lattice "$@"
