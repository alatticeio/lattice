#!/bin/bash

# ================= 配置区 =================
CLUSTER_NAME="wireflow"
GITHUB_RAW="https://raw.githubusercontent.com/wireflowio/wireflow/master"
# =========================================

set -e
# 颜色输出工具
info() { echo -e "\033[32m[INFO]\033[0m $1"; }
warn() { echo -e "\033[32m[INFO]\033[0m $1"; }
err() { echo -e "\033[31m[ERROR]\033[0m $1"; exit 1; }

info "🔍 开始系统环境自检..."

# 1. 安装 Docker (如果不存在)
if ! command -v docker &> /dev/null; then
    info "🐳 未检测到 Docker，准备安装..."
    curl -fsSL https://get.docker.com | bash
    # 启动并设置开机自启
    sudo systemctl enable --now docker
    # 允许当前用户操作 docker (可能需要重新登录生效，脚本内暂用 sudo 兜底)
    sudo usermod -aG docker $USER || true
    info "✅ Docker 安装完成"
else
    info "✅ Docker 已就绪: $(docker version --format '{{.Server.Version}}')"
fi

# 2. 确保 Docker 服务正在运行
if ! sudo docker ps > /dev/null 2>&1; then
    err "❌ Docker 服务未启动，请检查系统状态。"
fi

# 3. 安装 k3d (如果不存在)
if ! command -v k3d &> /dev/null; then
    info "📦 未检测到 k3d，正在安装..."
    curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
else
    info "✅ k3d 已就绪"
fi

# 4. 安装 kubectl (如果不存在)
if ! command -v kubectl &> /dev/null; then
    info "☸️ 未检测到 kubectl，正在下载..."
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x kubectl
    sudo mv kubectl /usr/local/bin/
fi

# 5. 创建/启动 k3d 集群
if k3d cluster list | grep -q "$CLUSTER_NAME"; then
    info "♻️ 集群 $CLUSTER_NAME 已存在，正在确保其处于运行状态..."
    k3d cluster start $CLUSTER_NAME
else
    info "🏗️ 正在创建 k3d 集群..."
    # 映射 WireGuard UDP 端口，并禁用自带的 Traefik 以释放资源
    k3d cluster create $CLUSTER_NAME \
        --servers 1 --agents 1 \
        -p "51820:51820/udp@agent:0" \
        --k3s-arg "--disable=traefik@server:0"
fi

# 6. 获取集群上下文
k3d kubeconfig merge $CLUSTER_NAME

# 7. 应用 GitHub 上的资源
info "📡 正在从 GitHub 同步并应用资源..."
kubectl apply -f "${GITHUB_RAW}/config/wireflow.yaml"

# 8. 最后验证
info "⏳ 等待 Control Plane 启动 (约 30s)..."
kubectl wait --for=condition=Ready pods --all -n default --timeout=60s || warn "部分 Pod 启动较慢，请稍后手动检查"

echo "------------------------------------------------"
info "🚀 所有组件部署完毕！"
echo -e "你可以使用以下命令检查你的用户态网络栈节点："
echo -e "\033[34mkubectl get nodes\033[0m"
echo -e "\033[34mkubectl get pods -A\033[0m"