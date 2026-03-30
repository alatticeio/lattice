# Blueprint: One-Click Quick Start (OSS)

## Objective
通过单个命令（如 `curl | sh`）在本地 k3d 环境中拉起完整的 Wireflow 管理面。

## Process Flow
1. **Pre-check**: 检查 Docker, k3d, kubectl 状态及端口（8080, 4222）占用情况。
2. **Cluster Creation**: 执行 `k3d cluster create wireflow`，配置 LoadBalancer 端口映射。
3. **Artifacts Loading**:
    - 导入 `wireflowd` 镜像。
    - 顺序加载 CRDs -> RBAC -> Service -> Deployment。
4. **Post-install**:
    - 探测 API 健康检查接口。
    - 生成并打印 Agent 的一键连接字符串（含 NATS 地址和初始 Token）。

## Success Criteria
- 用户在运行脚本 60 秒内能通过浏览器打开 UI。
- Agent 通过 `localhost:4222` 能直接完成握手。