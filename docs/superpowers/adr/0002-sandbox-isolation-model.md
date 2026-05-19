# Sandbox Isolation Model — SOCKS5 Sidecar First, runsc as Endgame

AI agent sandbox 的隔离模型选择：当前采用 **SOCKS5 sidecar 代理**模式做网络层隔离，预留 `gvisor`（runsc）和 `microvm`（Firecracker）作为更高安全等级的升级路径。

## Context

Lattice 需要为 AI agent 工作负载提供网络隔离能力：策略执行（egress CIDR 白名单）、审计日志（谁在什么时候访问了什么）、以及 overlay 网络接入。隔离等级从松到严有三个选项：SOCKS5 代理、gVisor runsc 容器、Firecracker MicroVM。

## Considered Options

### SOCKS5 Sidecar 代理（chosen）

Sandbox 作为独立进程运行，在主机网络（或 Pod 网络）上暴露一个 SOCKS5 代理端口。AI agent 进程通过 `ALL_PROXY=socks5://127.0.0.1:1080` 环境变量将出站 TCP 流量导向代理。代理内部走 gVisor 用户态 netstack → 策略检查 → 审计记录 → wireguard-go 加密 → Lattice overlay。

```
┌──────────────────────────────────────────┐
│  Pod                                     │
│                                          │
│  ┌──────────────┐  ┌──────────────────┐  │
│  │ AI Agent 容器  │  │ Sandbox 容器      │  │
│  │ ALL_PROXY=   │  │                  │  │
│  │ socks5://    │──│ :1080           │──│──▶ Lattice Overlay
│  │ localhost:1080│  │ 策略 ✓ 审计 ✓    │  │
│  └──────────────┘  └──────────────────┘  │
└──────────────────────────────────────────┘
```

- **优点**：零特权（无 `CAP_NET_ADMIN`，无 root），部署门槛极低（AI agent 无需容器化，任何语言、任何部署方式只需设环境变量），与 K8s sidecar 模式自然对齐
- **缺点**：代理是"自觉接入"——如果 AI agent 不使用 `ALL_PROXY`，流量绕过沙箱直连主机网络；DNS 可能泄漏；SOCKS5 仅覆盖 TCP

### gVisor runsc（升级路径，代码中预定义为 `SandboxGVisor`）

AI agent 进程运行在 gVisor sentry（用户态内核）内部，所有 syscall 被拦截。网络流量直接进入 gVisor 内嵌的 netstack，无需代理。

```
AI agent ──socket()──▶ gVisor sentry 拦截 ──▶ netstack ──▶ channel.Endpoint ──▶ wireguard-go ──▶ overlay
```

- **优点**：syscall 级强制，代理模型的所有逃逸路径（不走代理、DNS 泄漏、直连 socket）全部消除
- **缺点**：需要 runsc runtime 环境和容器化部署，需要 root 或 `CAP_SYS_ADMIN`，syscall 转译有约 5–15% 性能开销

### Firecracker MicroVM（未来，代码中预定义为 `SandboxMicroVM`）

AI agent 运行在完整的 Firecracker 微虚拟机中，硬件虚拟化级隔离。

- **优点**：近乎不可逃逸，适合运行不受信任的第三方 agent 代码
- **缺点**：完整 VM 开销（内存、启动时间），部署复杂度最高，当前未实现

## Decision

**分阶段递进，SOCKS5 sidecar 作为当前实现，runsc 和 microvm 作为预留升级路径。**

AgentIdentity CRD 中已定义三个 SandboxMode：

```go
SandboxPod     SandboxMode = "pod"      // 当前 SOCKS5 sidecar
SandboxGVisor  SandboxMode = "gvisor"   // runsc 用户态内核（Pro）
SandboxMicroVM SandboxMode = "microvm"  // Firecracker MicroVM（Pro，将来）
```

当前实现对应 `SandboxPod`。三个模式共享同一套底层组件：gVisor netstack、wireguard-go、channel endpoint。切换隔离模式时迁移成本低——只是"netstack 作为代理目标"变成"netstack 作为容器网卡"。

## Rationale

1. **零门槛接入是 AI agent 平台的核心竞争力。** SOCKS5 代理不需要用户改造 AI agent 的部署方式（裸机进程、Docker、K8s 都可以），不需要安装额外 runtime，不需要 root。这与 Lattice 宽泛的集成场景匹配。

2. **SOCKS5 是成熟模式。** 服务网格（Istio Envoy、Consul Connect）验证了 sidecar 代理架构。AI agent 场景比通用微服务更可控——平台控制 agent 的启动参数和环境变量，不存在"agent 不配合设 `ALL_PROXY`"的问题。

3. **策略和审计本身就构成产品价值。** 即使没有强制隔离，"哪个 agent 能访问什么"+"谁在什么时候访问了什么"，对 AI agent 平台已经是独立的产品功能点。先让这两个能力跑通，再叠加更强的隔离。

4. **从轻到重的路径清晰。** 每一步都在同一条架构线上升级：
   - 当前：SOCKS5 sidecar → 策略+审计跑通
   - 增强：加 iptables 透明重定向 → 强制拦截
   - 终局：runsc syscall 级隔离 → 不可绕过

## Consequences

- 文档和教程中需要明确告知用户 SOCKS5 代理模式的安全边界（不是完整的进程沙箱，代理可以被绕过）
- `ALL_PROXY` 环境变量的设置是平台接入的关键步骤，需要在各语言/框架的集成文档中覆盖
- 未来添加 iptables 透明重定向时，需要处理非 TCP 流量（DNS UDP、QUIC）的补漏
- runsc 模式实现时需要考虑与现有 K8s 部署方式的兼容性（可能需要 runsc runtimeClass + 特定 node 标签）

## runsc Implementation Approach（2026-05 更新）

### 方案选择：不内嵌 sentry，用原生 runsc + 现有网络栈

经过对 gVisor sentry 初始化流程的分析，**不采用"通过 NetworkBackend 注入自定义 netstack"的方案**。原因是：

- `boot.New()` 需要完整平台初始化（kvm/systrap/ptrace）、seccomp filter、内存文件（hugepages）、VDSO、watchdog 等，复杂度极高
- `inet.Stack` 接口有约 20 个方法需实现，实际是重写半个 gVisor
- gVisor 开发团队未将 sentry 作为公共嵌入 API 暴露

**最终方案：原生 runsc 做进程隔离 + 现有 lattice netstack 做网络代理。** runsc 以 `--network=none` 模式启动（仅 loopback，无外部网络），通过 `--pass-fd` 传入一个 Unix domain socket 连接到 host 端的 SOCKS5 代理。整个 lattice-shim、netstack、SOCKS5、ForwardListener、wireguard-go 全部复用，零改动。

### 出站和入站的分工

pod 模式下，agent 的网络流量有两条独立路径，由两个不同组件处理：

| 方向 | 谁发起 | 走哪个组件 | 为什么 |
|------|--------|-----------|--------|
| **出站** | agent 主动 `connect()` | Socks5Server | agent 通过 SOCKS5 CONNECT 告诉代理"我要连到 X"，代理代它建立 gVisor TCP 连接。响应数据沿同一条 SOCKS5 连接原路返回，不需要额外组件。 |
| **入站** | 外部 peer 主动连入 | ForwardListener | 外部不知道该 agent 有 SOCKS5 代理。ForwardListener 在 gVisor netstack 上监听 overlay 端口，收到连接后转发到 agent 的 host TCP 端口。 |

SOCKS5 是"正向代理"（agent → 代理 → 目标），ForwardListener 是"反向代理"（目标端口 → agent）。**两者不是上下游关系，是并行的两条独立管线**，共用同一个 gVisor netstack：

```
              ┌── SOCKS5（出站）: agent 发起 connect()
              │
  gVisor netstack
              │
              └── ForwardListener（入站）: 外部 peer 发起 connect()
```

runsc 模式保留这两个组件的分工不变，只是把 host TCP 接驳换成同一个 Unix socketpair。

### 数据流

两种模式共用同一个 gVisor netstack + WireGuard 管线，区别在于"agent 侧"的接驳方式。

#### pod 模式（当前）：Socks5Server / ForwardListener 与 host TCP 交互

```
  Socks5Server:                          ForwardListener:
  ┌──────────────────────┐              ┌───────────────────────┐
  │ accept(host TCP)     │              │ accept(gVisor TCP)    │
  │   ↓ SOCKS5 CONNECT   │              │   ↓                   │
  │ DialContext(gVisor)  │              │ net.Dial(host TCP)    │
  │   ↓                  │              │   ↓                   │
  │ relay(host ↔ gVisor) │              │ relay(gVisor ↔ host)  │
  └──────────────────────┘              └───────────────────────┘

  出站: agent → host TCP :1080 → Socks5Server → gVisor netstack → WG → overlay
  入站: overlay → WG → gVisor netstack → ForwardListener → host TCP → agent
```

#### runsc 模式（设计）：host TCP 替换为 socketpair

```
  Socks5Server:                          ForwardListener:
  ┌────────────────────────┐            ┌─────────────────────────┐
  │ accept(socketpair)     │            │ accept(gVisor TCP)  ← 不变
  │   ↓ SOCKS5 CONNECT     │            │   ↓                     │
  │ DialContext(gVisor) ← 不变          │ write(socketpair)       │  ← 替代 net.Dial
  │   ↓                    │            │   ↓                     │
  │ relay(sp ↔ gVisor)     │            │ relay(gVisor ↔ sp)      │
  └────────────────────────┘            └─────────────────────────┘

  出站: agent → fd 3 → socketpair → Socks5Server → gVisor netstack → WG → overlay
  入站: overlay → WG → gVisor netstack → ForwardListener → socketpair → fd 3 → agent
```

**关键：** gVisor netstack、EgressFilter、AuditWriter、channel.Endpoint、TUNAdapter、wireguard-go 在两种模式下路径完全一致。唯一变化是 agent 侧的接驳：Socks5Server 的 accept 来源从 `host TCP` 变为 `socketpair`，ForwardListener 的出方向从 `net.Dial(host TCP)` 变为 `write(socketpair)`。

### 组件复用

| 组件 | 当前 pod 模式 | runsc 模式 |
|------|-------------|-----------|
| `shim.Netstack` | 独立创建 | **复用** |
| `shim.Socks5Server` | TCP 监听 | **复用**（额外支持 Unix socket 监听） |
| `shim.ForwardListener` | overlay → host TCP | **复用**（写 socketpair host-end 代替 dial TCP） |
| `shim.EgressFilter` | CIDR 白名单 | **复用** |
| `shim.AuditWriter` | 审计日志 | **复用** |
| `gvisor.Sandbox` | netstack 包装 | **复用** |
| `gvisor.TUNAdapter` | channel ↔ wg | **复用** |
| `gvisor.SandboxProvisioner` | WG peer 管理 | **复用** |
| `agent.RegisterSandboxViaNATS` | NATS 注册 | **复用** |
| `agent.NewNode` | ICE/LRP/WG 节点 | **复用** |

**核心网络层组件在两种模式下完全一致，零改动。** ForwardListener 在 runsc 模式下通过同一个 socketpair 往 agent 写入站数据，替代 pod 模式下的 TCP dial。唯一新增的是 `internal/agent/runsc/` 包负责 runsc 进程生命周期管理 + Unix socketpair 传递。

### 安全边界

```
┌─────────────────────────────────────────────────────┐
│  gVisor sentry（隔离域）                              │
│                                                     │
│  AI Agent 进程                                       │
│    ├─ syscall 被 gVisor 拦截 ✓                       │
│    ├─ 文件系统受限（gofer + 只读 rootfs）✓             │
│    ├─ 只能通过注入的 Unix socket 通信 ✓               │
│    └─ 无法接触 WireGuard 私钥 ✓                      │
│                                                     │
│  即使是恶意 agent 也无法：                             │
│    ├─ 访问主机文件系统                                │
│    ├─ 绕过 SOCKS5 代理直连                            │
│    ├─ 窃取 WireGuard 私钥                            │
│    └─ 执行特权系统调用                                │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  Host（Lattice 进程）                                │
│                                                     │
│  WireGuard 私钥 ✓                                   │
│  EgressFilter 策略 ✓                                │
│  AuditWriter 审计 ✓                                 │
│  NATS JWT ✓                                        │
│                                                     │
│  这些关键凭据绝不会进入 runsc 容器                     │
└─────────────────────────────────────────────────────┘
```

### 关键文件变更

| 文件 | 变更 |
|------|------|
| `internal/agent/runsc/runsc.go` | **新建**：RunscManager 生命周期管理（Create/Start/Stop/Destroy），含 socketpair 创建和 OCI spec 生成 |
| `cmd/lattice/cmd/sandbox/sandbox_pro.go` | 新增 `--mode`、`--agent-rootfs`、`--agent-binary`、`--agent-args` flags；`runStart()` 加 gvisor 分支 |
| `lattice-shim/shim/socks5.go`（upstream） | 新增 `WithUnixListener` option，允许通过 socketpair 服务 SOCKS5 CONNECT |
| `lattice-shim/shim/forward.go`（upstream） | ForwardListener 新增写入 socketpair host-end 的能力（替代 TCP dial） |

### 与 pod 模式的对比

| | SOCKS5 Sidecar（pod） | gVisor runsc |
|---|---|---|
| **隔离层级** | 网络层（代理自觉接入） | 进程级（syscall 强制拦截） |
| **AI agent 如何接入** | 设置 `ALL_PROXY` 环境变量 | 无需配置，agent 进程跑在 gVisor sentry 里 |
| **能否绕过代理** | 可以（不设代理，直连） | 不可绕过（所有 syscall 被 gVisor 拦截） |
| **DNS 泄漏** | 可能 | 不可能 |
| **特权要求** | 零特权 | 需 runsc runtime 安装，以 root 运行 runsc |
| **性能开销** | 接近零 | syscall 转译约 5–15% |
| **适用场景** | 可信 agent、开发调试 | 不可信 agent、安全合规要求高的环境 |
