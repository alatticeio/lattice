#!/usr/bin/env sh
# Lattice Agent Quick Install Script
# Usage: curl -fsSL <SERVER_URL>/install.sh | sh -s -- --server <SERVER_URL> --token <TOKEN> --name <NODE_NAME>
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
  LATTICE_VERSION="$(curl -fsSL "${GITHUB_RELEASES}/latest" | grep -o '"tag_name":"[^"]*"' | cut -d'"' -f4)"
  if [ -z "$LATTICE_VERSION" ]; then
    echo "Failed to resolve latest version from GitHub Releases"
    exit 1
  fi
  echo "Latest version: ${LATTICE_VERSION}"
fi

# ── download binary ──────────────────────────────────────────────────────────
# GoReleaser produces: lattice_v1.0.0_linux_amd64.tar.gz
ARCHIVE_NAME="lattice_${LATTICE_VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="${GITHUB_RELEASES}/download/${LATTICE_VERSION}/${ARCHIVE_NAME}"

echo "Downloading Lattice ${LATTICE_VERSION} for ${OS}/${ARCH}..."
TMP_DIR="$(mktemp -d /tmp/lattice-install.XXXXXX)"
trap 'rm -rf "$TMP_DIR"' EXIT

if ! curl -fsSL --connect-timeout 30 -o "${TMP_DIR}/${ARCHIVE_NAME}" "$DOWNLOAD_URL"; then
  echo "Download failed from ${DOWNLOAD_URL}"
  echo "Check available releases at: ${GITHUB_RELEASES}"
  exit 1
fi

tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"
mkdir -p "$INSTALL_DIR"
mv "${TMP_DIR}/lattice" "${INSTALL_DIR}/lattice"
chmod +x "${INSTALL_DIR}/lattice"

echo "Installed ${INSTALL_DIR}/lattice"

# ── create config dir ────────────────────────────────────────────────────────
mkdir -p "$LATTICE_CONFIG_DIR"

# ── start agent ──────────────────────────────────────────────────────────────
if [ "$OS" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
  cat > /etc/systemd/system/lattice-agent.service <<SYSTEMD
[Unit]
Description=Lattice Agent
After=network.target

[Service]
ExecStart=${INSTALL_DIR}/lattice up --server-url "${SERVER_URL}" --token "${TOKEN}" --name "${NODE_NAME}" --save
Restart=on-failure
RestartSec=5s
Environment="LATTICE_CONFIG_DIR=${LATTICE_CONFIG_DIR}"

[Install]
WantedBy=multi-user.target
SYSTEMD
  systemctl daemon-reload
  systemctl enable --now lattice-agent
  echo "Lattice Agent started via systemd. Check: journalctl -u lattice-agent -f"
elif [ "$OS" = "darwin" ]; then
  cat > ~/Library/LaunchAgents/io.lattice.agent.plist <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>      <string>io.lattice.agent</string>
  <key>ProgramArguments</key>
  <array>
    <string>${INSTALL_DIR}/lattice</string>
    <string>up</string>
    <string>--server-url</string> <string>${SERVER_URL}</string>
    <string>--token</string>      <string>${TOKEN}</string>
    <string>--name</string>       <string>${NODE_NAME}</string>
    <string>--save</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>LATTICE_CONFIG_DIR</key>
    <string>${LATTICE_CONFIG_DIR}</string>
  </dict>
  <key>RunAtLoad</key>  <true/>
  <key>KeepAlive</key>  <true/>
</dict>
</plist>
PLIST
  launchctl load ~/Library/LaunchAgents/io.lattice.agent.plist
  echo "Lattice Agent started via launchd."
else
  echo "Run manually: ${INSTALL_DIR}/lattice up --server-url '${SERVER_URL}' --token '${TOKEN}' --name '${NODE_NAME}' --save"
fi

echo "Done. Node '${NODE_NAME}' should appear in the Dashboard within 30 seconds."
