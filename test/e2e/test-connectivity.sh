#!/bin/bash

# 参数：Namespace A, Namespace B, 以及测试时长
NS_A=${1:-"wireflow-test-a"}
NS_B=${2:-"wireflow-test-b"}
TIMEOUT=60

echo "--------------------------------------------------"
echo "🚀 开始 Wireflow 集成连通性测试"
echo "📍 源空间: $NS_A | 目标空间: $NS_B"
echo "--------------------------------------------------"

# 1. 确保测试 Pod 已就绪
wait_for_pod() {
    local ns=$1
    echo "⏳ 等待 $ns 中的测试 Pod 就绪..."
    kubectl run test-node -n "$ns" --image=busybox --restart=Never --labels="app=wf-test" -- sleep 3600 --dry-run=client -o yaml | kubectl apply -f - > /dev/null
    kubectl wait --for=condition=Ready pod/test-node -n "$ns" --timeout=30s
}

wait_for_pod "$NS_A"
wait_for_pod "$NS_B"

# 2. 获取目标 IP (从你的 CRD 状态中获取)
# 假设你的 CRD 叫 WireflowPeer，你可以根据实际名称修改
echo "🔍 正在从 CRD 获取虚拟 IP..."
start_time=$(date +%s)
while true; do
    # 这里尝试获取 status 里的虚拟 IP，请根据你的 CRD 结构调整 jsonpath
    TARGET_IP=$(kubectl get wireflowpeer -n "$NS_B" -o jsonpath='{.items[0].status.virtualIP}' 2>/dev/null)

    if [ ! -z "$TARGET_IP" ]; then
        echo "✅ 找到目标虚拟 IP: $TARGET_IP"
        break
    fi

    current_time=$(date +%s)
    if [ $((current_time - start_time)) -gt $TIMEOUT ]; then
        echo "❌ 错误: 在 ${TIMEOUT}s 内未能在 $NS_B 获取到虚拟 IP"
        exit 1
    fi
    echo "... 还在等待隧道分配 IP ..."
    sleep 5
done

# 3. 执行连通性校验 (Ping)
echo "📡 正在尝试跨空间 Ping: $NS_A -> $TARGET_IP"
# -c: 次数, -W: 等待超时
kubectl exec test-node -n "$NS_A" -- ping -c 5 -W 2 "$TARGET_IP"

if [ $? -eq 0 ]; then
    echo "--------------------------------------------------"
    echo "✨ [SUCCESS] 跨空间网络隧道连通性验证通过！"
    echo "--------------------------------------------------"
    exit 0
else
    echo "--------------------------------------------------"
    echo "🚨 [FAILURE] 连通性测试失败！请检查 Agent 日志。"
    echo "--------------------------------------------------"
    exit 1
fi