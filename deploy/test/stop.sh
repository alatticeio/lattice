#!/usr/bin/env bash
# stop.sh — 停止并清理 Wireflow 10 节点集成测试环境
#
# 用法:
#   ./stop.sh           # 停止容器，保留卷
#   ./stop.sh --clean   # 停止容器并删除卷（完全清理）

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"

CLEAN=false
if [[ "${1:-}" == "--clean" ]]; then
  CLEAN=true
fi

echo "==> 停止 Wireflow 10 节点测试环境"

if $CLEAN; then
  echo "    模式: 完全清理（删除容器 + 卷）"
  docker compose -f "$COMPOSE_FILE" down -v --remove-orphans
  echo "==> 已清理所有容器和卷"
else
  docker compose -f "$COMPOSE_FILE" down --remove-orphans
  echo "==> 已停止所有容器（卷保留）"
  echo "    如需完全清理，运行: ./stop.sh --clean"
fi
