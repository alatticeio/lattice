#!/usr/bin/env sh
# Lattice Sandbox Agent Install Script
# Usage:
#   curl -fsSL <SERVER_URL>/install-sandbox.sh | sh -s -- --server URL --token TOKEN --name NAME
#
# The sandbox requires the PRO edition binary (lattice-pro).
# It downloads from GitHub Releases and installs as a systemd service.
#
# Copyright 2026 The Lattice Authors.
# Licensed under the Apache License, Version 2.0

set -e

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
LATTICE_CONFIG_DIR="${LATTICE_CONFIG_DIR:-/etc/lattice}"
LATTICE_VERSION="${LATTICE_VERSION:-latest}"
GITHUB_RELEASES="https://github.com/alatticeio/lattice/releases"

# ── parse args ───────────────────────────────────────────────────────────────
SERVER_URL=""
TOKEN=""
NODE_NAME=""
PROXY_ADDR="${PROXY_ADDR:-127.0.0.1:1080}"
FORWARD_RULES="${FORWARD_RULES:-}"
EGRESS_ALLOW="${EGRESS_ALLOW:-}"
EGRESS_DEFAULT_DENY="${EGRESS_DEFAULT_DENY:-true}"

while [ $# -gt 0 ]; do
  case "$1" in
    --server)          SERVER_URL="$2"; shift 2 ;;
    --token)           TOKEN="$2";      shift 2 ;;
    --name)            NODE_NAME="$2";  shift 2 ;;
    --proxy-addr)      PROXY_ADDR="$2"; shift 2 ;;
    --forward)         FORWARD_RULES="${FORWARD_RULES:+$FORWARD_RULES,}$2"; shift 2 ;;
    --egress-allow)    EGRESS_ALLOW="$2"; shift 2 ;;
    --egress-deny)     EGRESS_DEFAULT_DENY="$2"; shift 2 ;;
    --no-proxy)        PROXY_ADDR=""; shift ;;
    --no-egress-deny)  EGRESS_DEFAULT_DENY="false"; shift ;;
    *)                 echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [ -z "$SERVER_URL" ] || [ -z "$TOKEN" ] || [ -z "$NODE_NAME" ]; then
  echo "Usage: install-sandbox.sh --server <SERVER_URL> --token <TOKEN> --name <NODE_NAME>"
  echo ""
  echo "Sandbox options:"
  echo "  --proxy-addr ADDR       SOCKS5 listen address (default 127.0.0.1:1080)"
  echo "  --no-proxy               Disable SOCKS5 proxy"
  echo "  --forward PORT:ADDR      Inbound forward rule (repeatable)"
  echo "  --egress-allow CIDRS     Comma-separated allowed CIDRs"
  echo "  --egress-deny true|false  Default-deny egress (default true)"
  exit 1
fi

# Validate NODE_NAME contains only safe characters
case "$NODE_NAME" in
  *[!a-zA-Z0-9._-]*)
    echo "Error: Node name must contain only letters, numbers, dots, underscores, and hyphens"
    exit 1
    ;;
esac

# ── detect OS / arch ─────────────────────────────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  linux) ;;
  *) echo "Sandbox currently supports Linux only. Use Docker on macOS: see deploy/docker/docker-compose.sandbox.yml"; exit 1 ;;
esac

# ── resolve version ──────────────────────────────────────────────────────────
if [ "$LATTICE_VERSION" = "latest" ]; then
  LATTICE_VERSION="$(curl -fsSL "${GITHUB_RELEASES}/latest" | grep -o '"tag_name":"[^"]*"' | cut -d'"' -f4)"
  if [ -z "$LATTICE_VERSION" ]; then
    echo "Failed to resolve latest version from GitHub Releases"
    exit 1
  fi
  echo "Latest version: ${LATTICE_VERSION}"
fi

# ── download PRO binary ──────────────────────────────────────────────────────
# GoReleaser produces: lattice-pro_v1.0.0_linux_amd64.tar.gz
ARCHIVE_NAME="lattice-pro_${LATTICE_VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="${GITHUB_RELEASES}/download/${LATTICE_VERSION}/${ARCHIVE_NAME}"

echo "Downloading Lattice Sandbox ${LATTICE_VERSION} for ${OS}/${ARCH}..."
TMP_DIR="$(mktemp -d /tmp/lattice-install.XXXXXX)"
trap 'rm -rf "$TMP_DIR"' EXIT

if ! curl -fsSL --connect-timeout 30 -o "${TMP_DIR}/${ARCHIVE_NAME}" "$DOWNLOAD_URL"; then
  echo "Download failed from ${DOWNLOAD_URL}"
  echo "Check available releases at: ${GITHUB_RELEASES}"
  exit 1
fi

tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"
mkdir -p "$INSTALL_DIR"
# The PRO binary inside the archive is named "lattice-pro"
mv "${TMP_DIR}/lattice-pro" "${INSTALL_DIR}/lattice"
chmod +x "${INSTALL_DIR}/lattice"

echo "Installed ${INSTALL_DIR}/lattice"

# ── create config dir ────────────────────────────────────────────────────────
mkdir -p "$LATTICE_CONFIG_DIR"

# ── build sandbox start args ─────────────────────────────────────────────────
SANDBOX_ARGS="sandbox start --name \"${NODE_NAME}\" --server-url \"${SERVER_URL}\" --token \"${TOKEN}\""

if [ -n "$PROXY_ADDR" ]; then
  SANDBOX_ARGS="$SANDBOX_ARGS --proxy-addr \"${PROXY_ADDR}\""
fi

if [ -n "$FORWARD_RULES" ]; then
  OLD_IFS="$IFS"
  IFS=","
  for rule in $FORWARD_RULES; do
    rule="$(echo "$rule" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    if [ -n "$rule" ]; then
      SANDBOX_ARGS="$SANDBOX_ARGS --forward \"${rule}\""
    fi
  done
  IFS="$OLD_IFS"
fi

if [ -n "$EGRESS_ALLOW" ]; then
  SANDBOX_ARGS="$SANDBOX_ARGS --egress-allow \"${EGRESS_ALLOW}\""
fi

if [ "$EGRESS_DEFAULT_DENY" = "true" ]; then
  SANDBOX_ARGS="$SANDBOX_ARGS --egress-default-deny"
fi

# ── start as systemd service ─────────────────────────────────────────────────
cat > /etc/systemd/system/lattice-sandbox.service <<SYSTEMD
[Unit]
Description=Lattice Sandbox Agent (${NODE_NAME})
After=network.target

[Service]
ExecStart=${INSTALL_DIR}/lattice ${SANDBOX_ARGS}
Restart=on-failure
RestartSec=5s
Environment="LATTICE_CONFIG_DIR=${LATTICE_CONFIG_DIR}"

[Install]
WantedBy=multi-user.target
SYSTEMD

systemctl daemon-reload
systemctl enable --now lattice-sandbox

echo ""
echo "=============================================="
echo " Lattice Sandbox is running!"
echo "=============================================="
echo ""
echo "  Name:       ${NODE_NAME}"
echo "  Server:     ${SERVER_URL}"
if [ -n "$PROXY_ADDR" ]; then
  echo "  SOCKS5:     ${PROXY_ADDR}"
  echo "  Usage:      ALL_PROXY=socks5://${PROXY_ADDR} your-ai-agent"
fi
echo ""
echo "  Status:     systemctl status lattice-sandbox"
echo "  Logs:       journalctl -u lattice-sandbox -f"
echo "  Audit:      /tmp/lattice-audit.jsonl"
echo ""
echo "Done."
