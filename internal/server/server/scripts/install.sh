#!/usr/bin/env sh
# Lattice Agent Quick Install Script
# Usage: curl -sSL https://get.lattice.io | sh -s -- --server <SERVER_URL> --token <TOKEN> --name <NODE_NAME>
#
# Copyright 2024 The Lattice Authors.
# Licensed under the Apache License, Version 2.0

set -e

LATTICE_VERSION="${LATTICE_VERSION:-latest}"
CDN_BASE="${CDN_BASE:-https://dl.lattice.io}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── parse args ───────────────────────────────────────────────────────────────
SERVER_URL=""
TOKEN=""
NODE_NAME=""

while [ $# -gt 0 ]; do
  case "$1" in
    --server) SERVER_URL="$2"; shift 2 ;;
    --token)  TOKEN="$2";      shift 2 ;;
    --name)   NODE_NAME="$2";  shift 2 ;;
    *)        echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [ -z "$SERVER_URL" ] || [ -z "$TOKEN" ] || [ -z "$NODE_NAME" ]; then
  echo "Usage: install.sh --server <SERVER_URL> --token <ENROLL_TOKEN> --name <NODE_NAME>"
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
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# ── resolve version ──────────────────────────────────────────────────────────
if [ "$LATTICE_VERSION" = "latest" ]; then
  LATTICE_VERSION="$(curl -fsSL "${CDN_BASE}/stable.txt")"
  if [ -z "$LATTICE_VERSION" ]; then
    echo "Failed to resolve latest version from ${CDN_BASE}/stable.txt"
    exit 1
  fi
fi

BINARY_URL="${CDN_BASE}/releases/${LATTICE_VERSION}/lattice-agent-${OS}-${ARCH}"
echo "Downloading Lattice Agent ${LATTICE_VERSION} for ${OS}/${ARCH}..."
TMP_FILE="$(mktemp /tmp/lattice-agent.XXXXXX)"
curl -fsSL -o "$TMP_FILE" "$BINARY_URL"
chmod +x "$TMP_FILE"
mv "$TMP_FILE" "${INSTALL_DIR}/lattice-agent"

echo "Installed to ${INSTALL_DIR}/lattice-agent"

# ── start agent ──────────────────────────────────────────────────────────────
if [ "$OS" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
  cat > /etc/systemd/system/lattice-agent.service <<EOF
[Unit]
Description=Lattice Agent
After=network.target

[Service]
ExecStart=${INSTALL_DIR}/lattice-agent up --server-url "${SERVER_URL}" --token "${TOKEN}" --name "${NODE_NAME}" --save
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now lattice-agent
  echo "Lattice Agent started via systemd. Run: journalctl -u lattice-agent -f"
elif [ "$OS" = "darwin" ]; then
  cat > ~/Library/LaunchAgents/io.lattice.agent.plist <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>      <string>io.lattice.agent</string>
  <key>ProgramArguments</key>
  <array>
    <string>${INSTALL_DIR}/lattice-agent</string>
    <string>up</string>
    <string>--server-url</string> <string>${SERVER_URL}</string>
    <string>--token</string>      <string>${TOKEN}</string>
    <string>--name</string>       <string>${NODE_NAME}</string>
    <string>--save</string>
  </array>
  <key>RunAtLoad</key>  <true/>
  <key>KeepAlive</key>  <true/>
</dict>
</plist>
EOF
  launchctl load ~/Library/LaunchAgents/io.lattice.agent.plist
  echo "Lattice Agent started via launchd."
else
  echo "Run manually: ${INSTALL_DIR}/lattice-agent up --server-url '${SERVER_URL}' --token '${TOKEN}' --name '${NODE_NAME}' --save"
fi

echo "Done. Your node '${NODE_NAME}' should appear in the Dashboard within 30 seconds."
