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
REPO="winstonfly/lattice"
GITHUB_API="https://api.github.com/repos/${REPO}/releases"
GITHUB_RELEASES="https://github.com/${REPO}/releases"

# ── parse args ───────────────────────────────────────────────────────────────
SERVER_URL=""
TOKEN=""
NODE_NAME=""
TAG=""

while [ $# -gt 0 ]; do
  case "$1" in
    --server) SERVER_URL="$2"; shift 2 ;;
    --token)  TOKEN="$2";      shift 2 ;;
    --name)   NODE_NAME="$2";  shift 2 ;;
    --tag)    TAG="$2";        shift 2 ;;
    *)        echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [ -z "$SERVER_URL" ] || [ -z "$TOKEN" ]; then
  echo "Usage: install.sh --server <SERVER_URL> --token <ENROLL_TOKEN> [--name <NODE_NAME>] [--tag <VERSION>]"
  exit 1
fi

# Auto-generate node name from hostname if not provided
if [ -z "$NODE_NAME" ]; then
  NODE_NAME="$(hostname -s 2>/dev/null || echo "node")-$$"
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
# --tag flag takes highest priority, then LATTICE_VERSION env var
if [ -n "$TAG" ]; then
  LATTICE_VERSION="$TAG"
fi

if [ "$LATTICE_VERSION" = "latest" ]; then
  # /releases/latest returns 404 for pre-releases; fall back to /releases list
  LATTICE_VERSION="$(curl -sSL "${GITHUB_API}/latest" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
  if [ -z "$LATTICE_VERSION" ]; then
    LATTICE_VERSION="$(curl -fsSL "${GITHUB_API}?per_page=1" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
  fi
  if [ -z "$LATTICE_VERSION" ]; then
    echo "Failed to resolve latest version from GitHub Releases. Use --tag to specify a version."
    exit 1
  fi
  echo "Latest version: ${LATTICE_VERSION}"
fi

# ── download binary ──────────────────────────────────────────────────────────
# GoReleaser produces: lattice_0.1.0-alpha_linux_amd64.tar.gz (no leading v)
VERSION_BARE="${LATTICE_VERSION#v}"
ARCHIVE_NAME="lattice_${VERSION_BARE}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="${GITHUB_RELEASES}/download/${LATTICE_VERSION}/${ARCHIVE_NAME}"

TMP_DIR="$(mktemp -d /tmp/lattice-install.XXXXXX)"
trap 'rm -rf "$TMP_DIR"' EXIT

CHECKSUMS_URL="${GITHUB_RELEASES}/download/${LATTICE_VERSION}/checksums.txt"
HASH_FILE="${INSTALL_DIR}/.lattice-archive-hash"

# sha256 helper: works on both Linux (sha256sum) and macOS (shasum -a 256)
sha256_file() { sha256sum "$1" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$1" | awk '{print $1}'; }

# Download checksums.txt (small) and extract expected hash for this archive.
EXPECTED_HASH=""
if curl -fsSL --connect-timeout 10 -o "${TMP_DIR}/checksums.txt" "$CHECKSUMS_URL" 2>/dev/null; then
  EXPECTED_HASH="$(grep " ${ARCHIVE_NAME}$" "${TMP_DIR}/checksums.txt" | awk '{print $1}' || true)"
fi

# If installed binary hash matches expected, skip download entirely.
if [ -x "${INSTALL_DIR}/lattice" ] && [ -n "$EXPECTED_HASH" ] && [ -f "$HASH_FILE" ]; then
  STORED_HASH="$(cat "$HASH_FILE" 2>/dev/null || true)"
  if [ "$STORED_HASH" = "$EXPECTED_HASH" ]; then
    echo "[INFO]  Skipping binary download, installed lattice matches hash"
    # jump straight to starting the agent
    SKIP_INSTALL=true
  fi
fi

if [ "${SKIP_INSTALL:-false}" = "false" ]; then
  echo "Downloading Lattice ${LATTICE_VERSION} for ${OS}/${ARCH}..."
  if ! curl -fL --progress-bar --connect-timeout 30 -o "${TMP_DIR}/${ARCHIVE_NAME}" "$DOWNLOAD_URL"; then
    echo "Download failed from ${DOWNLOAD_URL}"
    echo "Check available releases at: ${GITHUB_RELEASES}"
    exit 1
  fi

  # Verify hash if we have the expected value.
  if [ -n "$EXPECTED_HASH" ]; then
    ACTUAL_HASH="$(sha256_file "${TMP_DIR}/${ARCHIVE_NAME}")"
    if [ "$ACTUAL_HASH" != "$EXPECTED_HASH" ]; then
      echo "Hash mismatch! expected=${EXPECTED_HASH} got=${ACTUAL_HASH}"
      exit 1
    fi
    echo "[INFO]  Hash verified OK"
  fi

  tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"
  if [ ! -w "$INSTALL_DIR" ]; then
    sudo mkdir -p "$INSTALL_DIR"
    sudo mv "${TMP_DIR}/lattice" "${INSTALL_DIR}/lattice"
    sudo chmod +x "${INSTALL_DIR}/lattice"
    [ -n "$EXPECTED_HASH" ] && echo "$EXPECTED_HASH" | sudo tee "$HASH_FILE" > /dev/null
  else
    mkdir -p "$INSTALL_DIR"
    mv "${TMP_DIR}/lattice" "${INSTALL_DIR}/lattice"
    chmod +x "${INSTALL_DIR}/lattice"
    [ -n "$EXPECTED_HASH" ] && echo "$EXPECTED_HASH" > "$HASH_FILE"
  fi
  echo "Installed ${INSTALL_DIR}/lattice"
fi

# ── start agent in background ─────────────────────────────────────────────────
# WireGuard requires root to create tun devices and write to /var/run/wireguard/
LOG_FILE="/tmp/lattice-agent.log"

# Stop any existing lattice agent before starting a new one.
# Use -f to match the full command line so we only kill "lattice up ..."
# processes and not a co-located latticed server (whose argv[0] is also
# "lattice" when built from the same Docker image).
if sudo pkill -f "lattice up" 2>/dev/null; then
  echo "Stopped existing Lattice agent."
  sleep 1
fi

echo ""
echo "Starting Lattice agent (requires sudo for WireGuard)..."
sudo nohup "${INSTALL_DIR}/lattice" up \
  --server-url "${SERVER_URL}" \
  --token "${TOKEN}" \
  --name "${NODE_NAME}" \
  --save > "${LOG_FILE}" 2>&1 &
AGENT_PID=$!

# Give it a moment to detect early crashes
sleep 2
if ! sudo kill -0 "$AGENT_PID" 2>/dev/null; then
  echo "Agent failed to start. Check logs: $LOG_FILE"
  tail -20 "$LOG_FILE" >&2
  exit 1
fi

echo "Agent started (PID $AGENT_PID). Logs: $LOG_FILE"
echo "Done. Node '${NODE_NAME}' should appear in the Dashboard within 30 seconds."
