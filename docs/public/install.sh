#!/bin/bash
set -euo pipefail

# ─── Overridable: environment variables OR CLI flags ─────────────────────────
# --binary / BINARY      Binary to install: lattice (default) or latticed
# --tag / TAG            Specific version e.g. v0.2.0; auto-detected if not set
# --server / SERVER      Control plane URL; triggers non-interactive lattice init
# --token / TOKEN        Enrollment token; triggers non-interactive lattice init
# --install-dir / INSTALL_DIR  Installation directory, default /usr/local/bin
# ─────────────────────────────────────────────────────────────────────────────
BINARY="${BINARY:-lattice}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TAG="${TAG:-}"
SERVER="${SERVER:-}"
TOKEN="${TOKEN:-}"
REPO="winstonfly/lattice"

# Parse CLI flags (override env vars)
while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)
      [[ $# -lt 2 ]] && { echo "Error: --binary requires a value" >&2; exit 1; }
      BINARY="$2"; shift 2 ;;
    --tag)
      [[ $# -lt 2 ]] && { echo "Error: --tag requires a value" >&2; exit 1; }
      TAG="$2"; shift 2 ;;
    --server)
      [[ $# -lt 2 ]] && { echo "Error: --server requires a value" >&2; exit 1; }
      SERVER="$2"; shift 2 ;;
    --token)
      [[ $# -lt 2 ]] && { echo "Error: --token requires a value" >&2; exit 1; }
      TOKEN="$2"; shift 2 ;;
    --install-dir)
      [[ $# -lt 2 ]] && { echo "Error: --install-dir requires a value" >&2; exit 1; }
      INSTALL_DIR="$2"; shift 2 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

# Validate --binary value
if [[ "$BINARY" != "lattice" && "$BINARY" != "latticed" ]]; then
  echo "Error: --binary must be 'lattice' or 'latticed', got: $BINARY" >&2
  exit 1
fi

# ─── Check dependencies ───────────────────────────────────────────────────────
for cmd in curl tar; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "Error: $cmd not found. Please install it and try again." >&2
    exit 1
  fi
done

# ─── Detect OS / Arch ────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture $ARCH" >&2
    exit 1
    ;;
esac

if [ "$OS" != "linux" ] && [ "$OS" != "darwin" ]; then
  echo "Error: unsupported operating system $OS" >&2
  exit 1
fi

# latticed is only published for linux
if [ "$BINARY" = "latticed" ] && [ "$OS" != "linux" ]; then
  echo "Error: latticed (all-in-one control plane) is only supported on linux, current OS is $OS" >&2
  exit 1
fi

# ─── Resolve version ─────────────────────────────────────────────────────────
if [ -z "${TAG:-}" ]; then
  echo "No version specified, fetching latest release from GitHub..."
  # /releases/latest only returns stable releases; prereleases (alpha/beta) return 404.
  # Fall back to /releases list which includes prereleases.
  if command -v jq &>/dev/null; then
    # Use -sSL (no -f) so a 404 returns the body instead of aborting with set -e
    TAG=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -r '.tag_name // empty')
    if [ -z "$TAG" ]; then
      TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=1" | jq -r '.[0].tag_name // empty')
    fi
  else
    TAG=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
    if [ -z "$TAG" ]; then
      TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=1" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
    fi
  fi
  if [ -z "$TAG" ]; then
    echo "Error: could not determine latest version. Please set TAG=v0.x.x manually." >&2
    exit 1
  fi
fi

VERSION="${TAG#v}"

# ─── Build download URL ───────────────────────────────────────────────────────
if [ "$BINARY" = "latticed" ]; then
  ARCHIVE_NAME="latticed_${VERSION}_${OS}_${ARCH}.tar.gz"
else
  ARCHIVE_NAME="lattice_${VERSION}_${OS}_${ARCH}.tar.gz"
fi

BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
URL="${BASE_URL}/${ARCHIVE_NAME}"
CHECKSUM_URL="${BASE_URL}/checksums.txt"

# ─── Download ─────────────────────────────────────────────────────────────────
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading $BINARY $TAG (${OS}/${ARCH})..."
echo "  URL: $URL"

if ! curl -fSL --progress-bar "$URL" -o "$TMP_DIR/$ARCHIVE_NAME"; then
  echo "Error: download failed. Check the version and your network connection." >&2
  exit 1
fi

# ─── Verify checksum (optional, skipped if file not found) ───────────────────
if curl -fsSL "$CHECKSUM_URL" -o "$TMP_DIR/checksums.txt" 2>/dev/null; then
  echo "Verifying checksum..."
  if command -v sha256sum &>/dev/null; then
    (cd "$TMP_DIR" && grep "$ARCHIVE_NAME" checksums.txt | sha256sum -c -)
  elif command -v shasum &>/dev/null; then
    (cd "$TMP_DIR" && grep "$ARCHIVE_NAME" checksums.txt | shasum -a 256 -c -)
  fi
fi

# ─── Extract & install ────────────────────────────────────────────────────────
tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"

if [ ! -f "$TMP_DIR/$BINARY" ]; then
  echo "Error: binary $BINARY not found in archive." >&2
  exit 1
fi

chmod +x "$TMP_DIR/$BINARY"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  echo "Sudo required to install $BINARY to $INSTALL_DIR..."
  sudo mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo ""
echo "✅ $BINARY $TAG installed → $INSTALL_DIR/$BINARY"
echo "   Run '$BINARY --version' to verify."

# ─── Auto-enrollment (when --server and --token are provided) ─────────────────
if [ -n "$SERVER" ] && [ -n "$TOKEN" ]; then
  echo ""
  echo "Auto-enrolling into $SERVER ..."
  if ! "$INSTALL_DIR/$BINARY" init --server "$SERVER" --token "$TOKEN"; then
    echo "Error: enrollment failed. Check --server URL and --token value." >&2
    exit 1
  fi
  echo "Starting agent..."
  "$INSTALL_DIR/$BINARY" up --detach
  echo ""
  echo "Agent is running. Run '$BINARY status' to check connectivity."
fi
