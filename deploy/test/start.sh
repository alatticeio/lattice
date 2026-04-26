#!/usr/bin/env bash
# start.sh — 启动 Wireflow 10 节点集成测试环境
#
# 用法:
#   ./start.sh [选项]
#
# 环境变量（必填）:
#   WF_TOKEN          workspace enrollment token
#   WF_SERVER_URL     management server 地址，如 http://192.168.1.10:8080
#   WF_SIGNALING_URL  signaling/NATS 地址，如 nats://192.168.1.10:4222
#   VM_ENDPOINT       VictoriaMetrics 地址，如 http://192.168.1.10:8428
#
# 环境变量（可选）:
#   WF_IMAGE          节点镜像，默认 ghcr.io/wireflowio/wireflowd:latest
#
# 示例:
#   WF_TOKEN=abc123 \
#   WF_SERVER_URL=http://192.168.1.10:8080 \
#   WF_SIGNALING_URL=nats://192.168.1.10:4222 \
#   VM_ENDPOINT=http://192.168.1.10:8428 \
#   ./start.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"

# --- 必填变量检查 ---
missing=()
[[ -z "${WF_TOKEN:-}"          ]] && missing+=("WF_TOKEN")
[[ -z "${WF_SERVER_URL:-}"     ]] && missing+=("WF_SERVER_URL")
[[ -z "${WF_SIGNALING_URL:-}"  ]] && missing+=("WF_SIGNALING_URL")
[[ -z "${VM_ENDPOINT:-}"       ]] && missing+=("VM_ENDPOINT")

if [[ ${#missing[@]} -gt 0 ]]; then
  echo "错误: 以下环境变量未设置: ${missing[*]}"
  echo "用法示例:"
  echo "  WF_TOKEN=xxx WF_SERVER_URL=http://host:8080 WF_SIGNALING_URL=nats://host:4222 VM_ENDPOINT=http://host:8428 ./start.sh"
  exit 1
fi

echo "==> 启动 Wireflow 10 节点测试环境"
echo "    IMAGE:        ${WF_IMAGE:-ghcr.io/wireflowio/wireflowd:latest}"
echo "    SERVER_URL:   $WF_SERVER_URL"
echo "    SIGNALING_URL:$WF_SIGNALING_URL"
echo "    VM_ENDPOINT:  $VM_ENDPOINT"
echo ""

docker compose -f "$COMPOSE_FILE" up -d --remove-orphans

echo ""
echo "==> 节点启动完成，查看日志:"
echo "    docker compose -f $COMPOSE_FILE logs -f"
echo ""
echo "==> 验证指标上报:"
echo "    go run ../../hack/verify_metrics/main.go --vm-url $VM_ENDPOINT"
