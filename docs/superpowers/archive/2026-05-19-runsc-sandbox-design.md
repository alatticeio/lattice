# runsc Sandbox 设计：gVisor 全隔离方案

> 2026-05-19

## 背景

Lattice sandbox 现有的 pod 模式通过 SOCKS5 sidecar 做网络层隔离，但 SOCKS5 是「自觉接入」——AI agent 不设代理就能绕过。runsc 模式的目标是：

1. **syscall 级强制隔离**：gVisor sentry 拦截所有系统调用，AI agent 无法逃逸
2. **真正的网络隔离**：AI agent 有 overlay IP，只能访问 WireGuard overlay 网络，物理上无法绕过
3. **对 AI agent 完全透明**：AI agent 直接 `connect(peer-ip:port)`，无感知代理或 WireGuard

## 核心设计：Lattice agent 是容器基础设施

```
┌─────────────────────────────────────────────────────────────────┐
│  runsc container（gVisor sentry）                                │
│                                                                 │
│  PID 1: lattice sandbox agent                                   │
│    ① NATS 注册，拿 overlay IP（如 10.0.1.5）                    │
│    ② wireguard-go + /dev/net/tun → 建 wg0                      │
│    ③ 配路由：<overlay-cidr> → wg0，default → eth0               │
│    ④ 丢弃 CAP_NET_ADMIN inheritable set（prctl）                │
│    ⑤ syscall.Exec(agentBinary, agentArgs)                       │
│                                                                 │
│  AI agent（exec 后，零特权）                                     │
│    connect(10.0.1.5:8080)                                       │
│    → gVisor netstack → wg0 → wireguard-go → WireGuard → overlay │
└─────────────────────────────────────────────────────────────────┘
```

Lattice agent 作为 PID 1 把 overlay 网络搭建好，然后 exec AI agent。AI agent 启动时网络已就绪，直接使用，不感知任何基础设施细节。

## 网络拓扑

### gVisor 内部两张"网卡"

| 接口 | IP | 用途 |
|------|-----|-----|
| `wg0` | overlay IP（如 10.0.1.5） | AI agent 流量出口，wireguard-go 持有 TUN fd |
| `eth0` | host 分配 | WireGuard UDP 加密包出口，gVisor sandbox 外部接口 |

### 路由表

```
<overlay-cidr>  dev wg0   ← AI agent 的 overlay 流量
0.0.0.0/0       dev eth0  ← WireGuard UDP transport
```

### 流量路径（出站）

```
AI agent connect(peer-ip:port)
  → gVisor sentry 拦截 syscall
  → gVisor 内部 netstack 组 TCP 包
  → 路由：overlay-cidr → wg0
  → wireguard-go 从 wg0 TUN fd 读包，加密封装 UDP
  → UDP 包经 gVisor netstack → eth0 → host TUN → 公网
  → WireGuard peer 解密，还原原始包
```

### 隔离保证

AI agent 无法绕过 WireGuard，两层保证：
- **路由层**：gVisor netstack 里没有明文出公网的路由，`0.0.0.0/0` 只给 WireGuard UDP 用
- **syscall 层**：gVisor sentry 拦截所有 syscall，AI agent 无法建 raw socket、修改路由

## CAP_NET_ADMIN 的处理

`wireguard-go + /dev/net/tun` 创建 wg0 需要 `CAP_NET_ADMIN`。

在 gVisor 中，`CAP_NET_ADMIN` 是**虚拟化**的：

```
容器内 ioctl(TUNSETIFF)
  → gVisor sentry 拦截
  → 在 gVisor 内部 netstack 创建虚拟 TUN 接口
  → host kernel 完全不知道这件事
```

OCI spec 给容器赋予 `CAP_NET_ADMIN`，gVisor 允许其在内部 netstack 操作，不给予任何真实 host kernel 访问能力。这是 gVisor 的标准用法。

Lattice agent 在 `exec` AI agent 前用 `prctl(PR_SET_SECUREBITS)` 清空 inheritable capability set，AI agent 以零特权运行。

## 与 pod 模式的对比

| | pod 模式 | runsc 模式 |
|---|---|---|
| 隔离机制 | SOCKS5 代理（自觉接入） | gVisor sentry（syscall 强制） |
| 网络隔离 | 无（AI agent 在 host 网络，可绕过代理） | 有（只有 wg0 路由，物理不可绕过） |
| AI agent 接入方式 | 设 ALL_PROXY | 直接 connect()，透明 |
| CAP_NET_ADMIN | 不需要 | gVisor 虚拟化，安全 |
| wireguard-go TUN | channel.Endpoint（内存通道） | /dev/net/tun（gVisor 虚拟 TUN） |
| 适用场景 | 可信 agent，快速接入 | 不可信 agent，高安全要求 |

## 扩展性：IsolationDriver 接口

host 侧抽象 `IsolationDriver`，支持未来扩展：

```go
// driver.go（无 build tag）
type DriverConfig struct {
    SandboxName  string
    ServerURL    string
    Token        string
    EgressAllow  string
    EgressDeny   bool
    ForwardRules []string  // pod 模式入站转发
    AgentBinary  string    // runsc 模式 AI agent 路径
    AgentArgs    []string
    RootFS       string    // runsc 模式容器 rootfs
    BundleDir    string
}

type IsolationDriver interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
    Done() <-chan struct{}
}
```

已有/未来实现：

| Driver | 模式 | 说明 |
|--------|------|------|
| `PodDriver` | `--mode pod` | 现有 SOCKS5 sidecar 逻辑提取 |
| `RunscDriver` | `--mode gvisor` | 新 gVisor 容器方案 |
| `FirecrackerDriver` | `--mode microvm` | 未来 Firecracker 方案 |

## 文件变更

### 新增

| 文件 | 说明 |
|------|------|
| `cmd/lattice/cmd/sandbox/driver.go` | `IsolationDriver` 接口 + `DriverConfig`（无 build tag） |
| `cmd/lattice/cmd/sandbox/driver_pod.go` | `PodDriver`（`//go:build pro`，从 `sandbox_pro.go` 提取） |
| `cmd/lattice/cmd/sandbox/driver_runsc.go` | `RunscDriver`（`//go:build pro`） |
| `cmd/lattice/cmd/sandbox/sandbox_agent.go` | `lattice sandbox agent` 子命令（`//go:build pro`），容器内 PID 1 逻辑 |

### 修改

| 文件 | 变更 |
|------|------|
| `cmd/lattice/cmd/sandbox/sandbox.go` | 注册 `agent` 子命令 |
| `cmd/lattice/cmd/sandbox/sandbox_pro.go` | `runStart()` 重构为薄 orchestrator，委托给 driver |
| `internal/agent/runsc/runsc.go` | OCI spec 生成：改为 `--network=sandbox`，加 `CAP_NET_ADMIN`，移除 socketpair/pass-fd |

### 保留不动

| 文件 | 原因 |
|------|------|
| `internal/agent/runsc/socks5.go` | pod 模式继续使用 |
| `internal/agent/gvisor/` | pod 模式继续使用 |
| `internal/agent/runsc/runsc.go` Manager 结构 | 保留 Create/Start/Stop/Destroy，只改 OCI spec |

## sandbox_pro.go 重构后的结构

```
runStart()
  → 解析 flags，构建 DriverConfig
  → newDriver(sandboxMode, cfg) → IsolationDriver
  → driver.Start(ctx)
  → 监听 SIGINT/SIGTERM
  → driver.Stop()
```

`newDriver()` 工厂函数根据 `--mode` 返回对应 driver，新增模式只需实现接口、注册工厂，不动现有代码。

## sandbox_agent 子命令（容器内 PID 1）

```
lattice sandbox agent
  --name <name>
  --server-url <url>
  --token <token>
  [--egress-allow <cidrs>]
  [--egress-default-deny]
  -- <agent-binary> [agent-args...]
```

执行流程：

```
① agent.RegisterSandboxViaNATS()    // 复用现有代码
② agent.NewNode()                   // 无 CustomTUN，标准 wireguard-go + /dev/net/tun
③ node.Start() + StartHeartbeat()
④ 等待 WireGuard peers 就绪
⑤ 配 overlay 路由（netlink 或 ip 命令）
⑥ prctl 清空 inheritable capabilities
⑦ syscall.Exec(agentBinary, agentArgs, env)
```

步骤 ①-③ 全部复用现有 `sandbox_pro.go` 中的代码，搬移而非重写。

## 待验证项

`agent.NewNode()` 在不传 `CustomTUN` 时默认用 `tun.CreateTUN()`（标准 `/dev/net/tun` 路径）。需确认该路径在 gVisor sandbox 网络模式下工作正常：

- `open("/dev/net/tun")` → gVisor 拦截，在内部 netstack 创建虚拟 TUN ✓（gVisor 支持）
- wireguard-go UDP socket → gVisor eth0 → host TUN → 公网 ✓（gVisor sandbox 模式支持）
- `ip route add` 或 netlink → gVisor 拦截，修改内部路由表 ✓（gVisor 支持）

如 `agent.NewNode()` 的默认 TUN 路径不适用，需在 `sandbox_agent.go` 中显式用 `tun.CreateTUN("wg0", mtu)` 并传入 `CustomTUN`。
