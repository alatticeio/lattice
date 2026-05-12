# lattice-agent-sandbox 使用指南

> 关联设计: `docs/superpowers/specs/2026-05-11-agent-sandbox-and-ecosystem-design.md`

## 概述

`lattice-agent-sandbox` 是 Lattice 的零特权 AI Agent 沙箱启动器。它在 gVisor 用户态内核中运行 Agent 进程，用纯 Go netstack 替代内核 TUN 设备和 eBPF，**无需 root、CAP_NET_ADMIN 或内核模块**。

## 快速开始

### 1. 在控制面创建 enrollment token

```bash
curl -X POST http://localhost:8080/api/v1/agent-isolation/enrollment-tokens \
  -H "Authorization: Bearer <user-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "default",
    "allowedTools": ["list_peers", "list_policies", "check_connectivity"],
    "ttlSeconds": 3600
  }'
```

返回:
```json
{
  "code": 200,
  "data": {
    "Token": "a1b2c3...",
    "ExpiresAt": "2026-05-12T14:00:00Z"
  }
}
```

### 2. 启动 sandbox agent

**自动注册模式**（推荐）:

```bash
lattice-agent-sandbox start \
  --name my-agent \
  --server-url http://localhost:8080 \
  --token a1b2c3...
```

流程:
1. 本地生成 WireGuard 密钥对
2. 调用 `POST /api/v1/agent-isolation/register` 注册 —— 服务端创建 `LatticePeer` + `AgentIdentity` (Sandbox=gvisor)，签发 Agent JWT
3. 获取分配的 VPN IP（如 `10.100.0.5`）
4. 创建 gVisor sandbox：`shim.Netstack` + 策略/审计 hook
5. 阻塞等待 SIGTERM

**指定 IP 模式**（跳过注册）:

```bash
lattice-agent-sandbox start \
  --name my-agent \
  --local-ip 10.100.0.5
```

不调控制面，直接创建 sandbox。适用于已有 IP 或纯本地测试。

### 3. 启用 WireGuard 隧道（待实现）

```bash
lattice-agent-sandbox start \
  --name my-agent \
  --local-ip 10.100.0.5 \
  --wg
```

`--wg` 会将 gVisor channel endpoint 桥接到 wireguard-go，使 sandbox 内流量经 WireGuard 隧道与其他 peer 通信。

## 命令行参考

```
lattice-agent-sandbox start [flags]

Flags:
  --name string         Sandbox 标识符（必填）
  --mode string         隔离模式: gvisor（默认）
  --local-ip string     VPN IP 地址。若为空且指定了 --server-url，注册后自动获取
  --wg                  启用 WireGuard 隧道附着
  --server-url string   Lattice 控制面 URL（用于自动注册）
  --token string        Enrollment token（用于自动注册）
```

## 流量模型

```
Agent 进程 (Python/Node/Go)
  → connect("api.internal", 443)
    → gVisor Sentry netstack (纯用户态，零特权)
      → shim.PolicyChecker.Allow() ← 策略注入点
        → shim.AuditWriter.Write() ← 审计注入点
          → wireguard-go (可选, --wg)
            → UDP :51820 → FilteringUDPMux → P2P/Relay
```

## 审计

gVisor 模式下审计由 `shim.AuditWriter` 接口捕获，不依赖 eBPF。每个 dial 操作产生一条事件:

```json
{
  "identity": "my-agent",
  "dst_ip": "10.100.0.2",
  "dst_port": 443,
  "protocol": "tcp",
  "verdict": "allow"
}
```

当前实现（`logAuditAdapter`）将事件写入 stderr。生产环境通过 `AuditAdapter` 批量上报控制面 `POST /api/v1/audit/batch`。

## API 接口

### 管理面（需用户 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agent-isolation/enrollment-tokens` | 创建一次性 enrollment token |
| POST | `/api/v1/agent-isolation/register` | Agent 注册（exchange token → JWT） |
| DELETE | `/api/v1/agent-isolation/agents/:name?namespace=` | 吊销 Agent |

### Agent 面（需 Agent JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agents/tools/call` | Agent 工具调用（经过 RBAC 检查） |

### 注册请求/响应

**请求**:
```json
{
  "enrollmentToken": "a1b2c3...",
  "agentName": "my-agent",
  "publicKey": "<wireguard-public-key-hex>",
  "sandbox": "gvisor"
}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "JWT": "eyJhbG...",
    "AgentIdentityName": "my-agent"
  }
}
```

## 与基础设施节点的对比

| | lattice（基础设施） | lattice-agent-sandbox（Agent 沙箱） |
|---|---|---|
| 隔离方式 | 无（宿主机进程） | gVisor 用户态内核 |
| 特权需求 | root / CAP_NET_ADMIN | **零**（普通用户） |
| 网络栈 | 内核 TUN (wf0) + WireGuard | gVisor Go netstack + wireguard-go |
| 策略执行 | eBPF TC ingress (PRO) / iptables | Go 层 PolicyChecker |
| 审计 | eBPF ring buffer (PRO) | Go 层 AuditWriter |
| 性能 | 高（内核态） | 中等（用户态，够用） |

## 构建

```bash
# 社区版（gVisor 不可用）
go build ./cmd/lattice-agent-sandbox/

# PRO 版（gVisor 可用）
go build -tags pro ./cmd/lattice-agent-sandbox/
```

## 限制

- gVisor 实现了 ~95% 常用 Linux syscall，部分应用可能不兼容（如依赖 `io_uring` 的程序）
- WireGuard 隧道附着（`--wg`）尚未完成 wireguard-go bind 层对接
- PolicyAdapter 当前使用 allow-all，策略引擎对接待完成
