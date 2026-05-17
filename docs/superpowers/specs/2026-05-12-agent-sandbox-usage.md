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

---

## 架构讨论：gVisor 网络原理与当前实现现状

> 记录时间：2026-05-16。来源：与 Claude 的架构讨论。

### gVisor 是什么，一般怎么用

gVisor 是 Google 开源的**用户空间内核**，核心是用 `sentry` 进程拦截容器内所有 syscall，让进程无法直接触碰宿主机 Linux 内核。有两种典型用法：

**用法 A：替换整个容器内核（`runsc` 模式）**

容器内所有进程的 syscall 都被 sentry 拦截，包括网络调用。Google Cloud Run、GitHub Actions runner 用的就是这种。

```bash
docker run --runtime=runsc untrusted-user-code
```

**用法 B：只用 gVisor 的网络栈（嵌入式 netstack）**

把 `pkg/tcpip`（gVisor 的 TCP/IP 实现）单独拿出来用，作为用户空间的网络栈，不需要内核 TUN 设备。Tailscale 的 userspace 模式就是这个路子。

**Lattice sandbox 用的是用法 B。** gVisor 在这里的作用是：用纯 Go 实现的 TCP/IP 替代内核网络栈，让 wireguard-go 不需要 `CAP_NET_ADMIN` 就能运行。

### gVisor 容器之间如何通信

gVisor（`runsc` 模式）**不替换容器间的网络基础设施**，CNI（Calico/Flannel/Cilium）和 veth pair、bridge 照常工作。gVisor 通过 **TUN 设备**把自己的 netstack 接入宿主机网络，出了 TUN 之后就是普通的 Linux 网络包。

```
Container A (gVisor)          Container B (gVisor)
  gVisor netstack               gVisor netstack
       ↓ TUN                         ↑ TUN
   veth pair                    veth pair
       ↓                             ↑
       └────── bridge (CNI) ─────────┘
               Linux kernel 负责转发
```

跨主机时，CNI 插件（VXLAN/BGP）负责节点间传输，gVisor 对此完全透明。

### Lattice sandbox 的网络通信现状

Sandbox 二进制做的事情：
1. 向控制面注册 → 拿 overlay IP（如 `10.0.0.3`）
2. 创建 gVisor netstack，localIP = 10.0.0.3
3. 创建 wireguard-go，通过 `tunAdapter`（而不是内核 TUN）桥接 gVisor channel endpoint
4. 配置 WireGuard peer（companion agent，endpoint = compPodIP:51820）
5. 启动 HTTP forward proxy（`127.0.0.1:1080`），代理通过 `sb.DialContext` 走 gVisor → WireGuard → overlay
6. 等待 SIGTERM

**通信能力现状：**

| 方向 | 实现方式 | 现状 |
|------|---------|------|
| Sandbox → 其他 Agent（出站） | HTTP proxy → `sb.DialContext` → gVisor → WG | ✅ 已实现，需设置 `http_proxy` |
| 其他 Agent → Sandbox（入站） | `sb.ListenTCP` | ❌ 未实现，无任何监听 |
| Sandbox → 外部互联网 | gVisor 无外部路由 | ❌ 未实现 |
| AI 代码进程网络隔离 | 进程直接 `connect()` 仍走 `eth0` | ❌ 可绕过 proxy |

### gVisor 在这里的真实角色

**不是**真正的沙箱（不拦截 AI 代码的 syscall），**而是**：

> 一个零特权的 WireGuard 网络栈载体——用 `pkg/tcpip` 替代内核 TCP/IP，让 wireguard-go 不需要 `CAP_NET_ADMIN` 即可运行；策略和审计作为附带能力，但只对主动经过 proxy 的流量有效。

要实现真正的 Agent 网络隔离，需要补齐：
1. **入站监听**：`sb.ListenTCP` 让其他 agent 能主动连接 sandbox
2. **强制代理**：切断 AI 代码到 `eth0` 的直连（iptables 或 netns 隔离），迫使所有流量走 proxy
3. **外部访问控制**：在 gVisor policy 层决定哪些外部地址可达

---

## 架构讨论：Sandbox 统一到 NewNode + NATS + ICE/LRP 基础设施

> 记录时间：2026-05-17。来源：实现重构后的架构审查讨论。

### 重构背景

原 sandbox 实现使用自定义的 WireGuard + NATS 启动逻辑，与普通 agent 的 `NewNode` 管道完全独立。重构目标：**让 sandbox 成为一等节点**，与普通 agent 共享同一套 NATS 信令 + ICE/LRP 基础设施，gVisor 只处理 WireGuard I/O、策略和审计。

### 当前架构（重构后）

Sandbox 现在完整走 `agent.NewNode` 三个阶段：

| 阶段 | 普通 Agent | Sandbox |
|------|-----------|---------|
| Phase 1 | 创建内核 TUN + UDP | 使用 gVisor TUNAdapter（`CustomTUN`） |
| Phase 1 | 连接 NATS，创建 `NatsService` | **完全相同** |
| Phase 2 | `ctrClient.Register()` → 从 server 取身份 | 跳过（`CurrentPeer != nil`，已 HTTP 预注册） |
| Phase 2 | 创建 `ProbeFactory`（ICE 信令） | **完全相同** |
| Phase 2 | 订阅 `lattice.signals.peers.<id>` | **完全相同** |
| Phase 3 | 创建 wireguard-go Device | **完全相同** |
| Phase 3 | 创建内核 `Provisioner`（iptables/eBPF） | 使用 `SandboxProvisioner`（wireguard-go IpcSet） |

ICE hole-punching、网络地图轮询、peer 发现全部走标准 NATS 信令路径，与普通 agent 无区别。

### 为什么 Sandbox 需要额外的 HTTP 注册步骤？

Sandbox 在调用 `NewNode` 之前需要完成两步：

1. `POST /api/v1/agent-isolation/register`（HTTP）
2. `fetchPeerViaNATS`：通过 NATS 轮询 `GetNetMap` 直到 VPN IP 分配完成

**这不是因为"拿 IP"的机制不同**（普通 agent 的 NATS Register 也拿不到即时分配的 IP，同样依赖 K8s controller 异步赋值，内部也要轮询 `GetNetMap`）。根本原因是 sandbox 的**身份模型与普通 agent 不同**：

| 差异点 | 普通 Agent（NATS Register） | Sandbox（HTTP Register） |
|--------|---------------------------|------------------------|
| Token 类型 | `LatticeEnrollmentToken`（K8s CRD） | `AgentEnrollmentToken`（SQLite DB） |
| 私钥存储 | server 生成并存入 `LatticePeer.Spec.PrivateKey` | **sandbox 本地生成**，server 只存公钥 |
| 额外 CRD | 只创建 `LatticePeer` | 还要创建 `AgentIdentity`（权限、工具列表、sandbox 模式、审计级别） |
| 返回内容 | `*infra.Peer`（含私钥） | JWT（用于后续 NATS 鉴权） |

sandbox 本地生成私钥是有意为之的安全决策：私钥永远不离开 sandbox 进程，server 无法得知私钥。

### 为什么不把两条路径合并？

理论上可以在 NATS Register 路径上支持"外部生成公钥 + 创建 AgentIdentity + 返回 JWT"，但需要改动：

- server 端 NATS handler（接收公钥字段）
- token 类型（统一 `LatticeEnrollmentToken` 和 `AgentEnrollmentToken`）
- K8s resource client（同时创建 `LatticePeer` + `AgentIdentity`）
- `NewNode` 的 Phase 2（处理 JWT 返回而非直接的 Peer 结构体）

目前的分离是合理的：HTTP 注册路径正是因为 sandbox 有独立的身份模型（AgentIdentity）而存在，强行合并会让普通 agent 的注册路径背上 sandbox 专属逻辑。`fetchPeerViaNATS` 的轮询与普通 agent 内部的 `GetNetMap` 重试在语义和开销上完全等价。

### NodeConfig 扩展点

为支持 sandbox 接入 `NewNode`，`NodeConfig` 新增了四个字段：

```go
// CustomTUN 替换内核 TUN，sandbox 传入 gVisor TUNAdapter。
CustomTUN tun.Device

// CustomName 替代从内核 TUN 设备读取的接口名。
CustomName string

// CurrentPeer 跳过 Phase 2 的 NATS Register 调用，直接使用预注册身份。
CurrentPeer *infra.Peer

// ProvisionerFactory 在 Phase 3 wireguard-go Device 创建后被调用，
// 返回自定义 Provisioner。nil 时使用默认内核 Provisioner（iptables/eBPF）。
ProvisionerFactory func(dev *wg.Device) provision.Provisioner
```

`SandboxProvisioner`（`internal/agent/gvisor/provisioner.go`，`//go:build pro`）实现 `provision.Provisioner`：WireGuard peer 操作路由到 `device.IpcSet()`，路由/IP/policy/NAT 操作均为 no-op（由 gVisor netstack 处理）。
