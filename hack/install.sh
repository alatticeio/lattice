#!/bin/bash
set -euo pipefail

# ─── 可覆盖的环境变量 ────────────────────────────────────────────────────────
# BINARY      要安装的二进制名称：lattice（边缘 Agent）或 latticed（All-in-One 控制面）
# INSTALL_DIR 安装目录，默认 /usr/local/bin
# TAG         指定版本，如 v0.2.0；不传则自动获取最新 Release
# ─────────────────────────────────────────────────────────────────────────────
BINARY="${BINARY:-lattice}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
REPO="alatticeio/lattice"

# ─── 检查依赖 ─────────────────────────────────────────────────────────────────
for cmd in curl tar; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "错误：未找到 $cmd，请先安装后重试。" >&2
    exit 1
  fi
done

# ─── 检测 OS / Arch ───────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "错误：不支持的架构 $ARCH" >&2
    exit 1
    ;;
esac

if [ "$OS" != "linux" ] && [ "$OS" != "darwin" ]; then
  echo "错误：不支持的操作系统 $OS" >&2
  exit 1
fi

# latticed 目前只发布 linux 版本
if [ "$BINARY" = "latticed" ] && [ "$OS" != "linux" ]; then
  echo "错误：latticed (all-in-one 控制面) 仅支持 linux，当前系统为 $OS" >&2
  exit 1
fi

# ─── 解析版本 ─────────────────────────────────────────────────────────────────
if [ -z "${TAG:-}" ]; then
  echo "未指定版本，正在从 GitHub 获取最新 Release..."
  API_URL="https://api.github.com/repos/${REPO}/releases/latest"
  # 优先用 jq；回退到 sed
  if command -v jq &>/dev/null; then
    TAG=$(curl -fsSL "$API_URL" | jq -r '.tag_name')
  else
    TAG=$(curl -fsSL "$API_URL" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
  fi
  if [ -z "$TAG" ]; then
    echo "错误：无法获取最新版本，请手动指定 TAG=v0.x.x" >&2
    exit 1
  fi
fi

VERSION="${TAG#v}"

# ─── 构造下载 URL ─────────────────────────────────────────────────────────────
# latticed 有独立归档；lattice / wfctl 共享同一个归档
if [ "$BINARY" = "latticed" ]; then
  ARCHIVE_NAME="latticed_${VERSION}_${OS}_${ARCH}.tar.gz"
else
  ARCHIVE_NAME="lattice_${VERSION}_${OS}_${ARCH}.tar.gz"
fi

BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
URL="${BASE_URL}/${ARCHIVE_NAME}"
CHECKSUM_URL="${BASE_URL}/checksums.txt"

# ─── 下载并安装 ───────────────────────────────────────────────────────────────
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "正在下载 $BINARY $TAG (${OS}/${ARCH})..."
echo "  URL: $URL"

if ! curl -fSL --progress-bar "$URL" -o "$TMP_DIR/$ARCHIVE_NAME"; then
  echo "错误：下载失败，请检查版本号和网络连接。" >&2
  exit 1
fi

# ─── 校验 checksum（可选，文件不存在时跳过）──────────────────────────────────
if curl -fsSL "$CHECKSUM_URL" -o "$TMP_DIR/checksums.txt" 2>/dev/null; then
  echo "正在校验文件完整性..."
  if command -v sha256sum &>/dev/null; then
    (cd "$TMP_DIR" && grep "$ARCHIVE_NAME" checksums.txt | sha256sum -c -)
  elif command -v shasum &>/dev/null; then
    (cd "$TMP_DIR" && grep "$ARCHIVE_NAME" checksums.txt | shasum -a 256 -c -)
  fi
fi

# ─── 解压 & 安装 ─────────────────────────────────────────────────────────────
tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"

if [ ! -f "$TMP_DIR/$BINARY" ]; then
  echo "错误：归档中未找到二进制文件 $BINARY" >&2
  exit 1
fi

chmod +x "$TMP_DIR/$BINARY"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  echo "需要 sudo 权限将 $BINARY 安装到 $INSTALL_DIR..."
  sudo mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo ""
echo "✅ $BINARY $TAG 安装成功 → $INSTALL_DIR/$BINARY"
echo "   运行 '$BINARY --version' 验证安装。"
