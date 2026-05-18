# Agent Sandbox 网络隔离与通信设计

> 日期：2026-05-16
> 关联：`2026-05-11-agent-sandbox-and-ecosystem-design.md`、`2026-05-12-agent-sandbox-usage.md`

## 背景与问题

当前 `lattice-agent-sandbox` 存在四个关键缺口：

| 缺口 | 描述 |
|------|------|
| **出站控制缺失** | PolicyChecker 当前 allow-all，无外部域名/IP 白名单 |
| **入站缺失** | 其他 agent 无法主动连接 sandbox |
| **动态 peer 管理缺失** | WireGuard peer 只能启动时静态配置，无法跟随网络拓扑变化 |
| **Workload 网络隔离缺失** | 容器内 AI 代码进程可绕过 proxy 直连 eth0，策略无效 |

---

## 架构总览

```
┌──────────────────────────────────────────────┐
│  Sandbox Pod                                  │
│                                               │
│  ┌──────────────────────────────────────┐    │
│  │  lattice-agent-sandbox 进程           │    │
│  │                                       │    │
│  │  ┌──────────┐  ┌──────────────────┐  │    │
│  │  │ shim.     │  │ WireGuardManager │  │    │
│  │  │ Netstack  │  │ (动态 peer 同步) │  │    │
│  │  └────┬─────┘  └────────┬─────────┘  │    │
│  │       │                 │             │    │
│  │  ┌────▼─────────────────▼──────────┐ │    │
│  │  │     wireguard-go (userspace)     │ │    │
│  │  └──────────────────────────────────┘│    │
│  │       ↑ UDP :51820                   │    │
│  │  ┌────┴──────┐  ┌───────────────┐   │    │
│  │  │HTTP Proxy  │  │ForwardListener│   │    │
│  │  │:1080 出站  │  │:overlay 入站  │   │    │
│  │  └────────────┘  └───────────────┘   │    │
│  └──────────────────────────────────────┘    │
│                                               │
│  ┌──────────────────────────────────────┐    │
│  │  Workload 容器（AI 代码/任意 agent）  │    │
│  │  网络被隔离，只能通过 proxy 通信      │    │
│  └──────────────────────────────────────┘    │
└──────────────────────────────────────────────┘
```

---

## 核心设计：lattice-shim 扩展

`lattice-shim` 作为通用网络库，为 sandbox 提供所有网络能力。当前 shim 已有：

- `shim.Netstack`：gVisor `pkg/tcpip` 用户态网络栈
- `shim.SOCKS5Server`：SOCKS5 出站代理（已实现）
- `shim.WireGuardEndpoint`：tunAdapter，连接 netstack ↔ wireguard-go
- `PolicyChecker` / `AuditWriter` hook

需要新增三个组件：

### 3.1 `shim.ForwardListener` — 入站端口转发

让其他 agent 能够通过 WireGuard overlay 主动连接 sandbox，并将连接转发到 workload 容器内部服务。

```go
// shim/forward.go

type ForwardRule struct {
    OverlayPort uint16  // 监听在 overlay IP 上的端口
    TargetAddr  string  // 转发目标，如 "127.0.0.1:8080"（workload 容器内）
}

type ForwardListener struct {
    ns    *Netstack
    rules []ForwardRule
}

// Start 在 overlay IP 上监听所有规则，将连接转发到 TargetAddr
func (f *ForwardListener) Start(ctx context.Context) error
```

数据流：
```
其他 Agent → WireGuard overlay → shim.ListenTCP(overlayIP:port)
  → ForwardListener → dial(targetAddr) → Workload 服务
```

### 3.2 `shim.EgressFilter` — 出站访问控制

在 `PolicyChecker` 上层提供域名/CIDR 白名单，per-workspace 策略从控制面下发。

```go
// shim/egress.go

type EgressPolicy struct {
    AllowedCIDRs   []net.IPNet  // 允许的 IP 段，如 overlay 网段
    AllowedDomains []string     // 允许的域名，如 "api.openai.com"
    DefaultDeny    bool         // true = 白名单模式
}

type EgressFilter struct {
    policy  atomic.Pointer[EgressPolicy]
    resolve func(domain string) ([]net.IP, error)
}

// Allow 实现 shim.PolicyChecker 接口
func (f *EgressFilter) Allow(identity, dstIP string, dstPort uint16) bool

// Update 热更新策略，无需重启
func (f *EgressFilter) Update(p EgressPolicy)
```

策略更新通过 NATS 信令推送，sandbox 进程订阅 `lattice.agent.{name}.policy` 主题，收到更新后调用 `EgressFilter.Update()`。

### 3.3 `shim.WireGuardManager` — 动态 peer 管理

订阅 NATS NetMap 变更，动态添加/移除 WireGuard peer，无需重启 sandbox。

```go
// shim/wgmanager.go

type WireGuardManager struct {
    wgDev    *wireguard.Device
    natsConn managementnats.Service
    agentJWT string
}

// Run 持续监听 NetMap 变更并同步 WireGuard peer 配置
func (m *WireGuardManager) Run(ctx context.Context) error

// syncPeers 对比当前 peer 列表和目标状态，添加/移除差异
func (m *WireGuardManager) syncPeers(netmap *infra.Message) error
```

---

## Workload 网络隔离

Workload 进程使用容器内核网络栈（eth0），默认可绕过 proxy 直连外部。隔离的目标是强制所有出站流量经过 sandbox 进程的 SOCKS5/HTTP proxy，再进入 gVisor → WireGuard 路径。

三种隔离方式按权限需求和隔离强度递增：

### Community 版：Linux Network Namespace + 路由截断

使用 initContainer 删除 eth0 默认路由，workload 进程找不到外部路由，只能走 proxy。

```yaml
initContainers:
- name: setup-netns
  image: busybox
  command:
  - sh
  - -c
  - |
    ip route del default       # 切断 eth0 直连外部
    # workload 只能访问 127.0.0.1（proxy 在此监听）
  securityContext:
    capabilities:
      add: [NET_ADMIN]
```

workload 容器设置：
```
http_proxy=http://127.0.0.1:1080
ALL_PROXY=socks5://127.0.0.1:1080
```

局限：workload 仍可直连同节点 Pod IP（未经过 overlay），需要结合网络策略补强。

### PRO 版 A：eBPF 透明代理（cgroup/connect hook）

在 sandbox 进程启动时，加载 eBPF `cgroup/connect4` 程序，拦截 workload 的所有 `connect()` 调用，透明重定向到本地 SOCKS5 proxy，**workload 无需配置任何环境变量**。

```
workload: connect(10.0.0.2:8080)
  → eBPF cgroup/connect4 hook 改写目标地址
    → connect(127.0.0.1:1080)  ← SOCKS5 proxy
      → sb.DialContext(10.0.0.2:8080)
        → gVisor netstack → wireguard-go → overlay
```

eBPF 程序挂载到 sandbox pod 的 cgroup，只影响该 pod 内进程，不影响宿主机。

```c
// internal/agent/ebpf/sandbox_tproxy.bpf.c
SEC("cgroup/connect4")
int sandbox_redirect(struct bpf_sock_addr *ctx) {
    // 非 loopback 目标 → 重定向到 SOCKS5 proxy
    if (!is_loopback(ctx->user_ip4)) {
        ctx->user_ip4   = bpf_htonl(0x7f000001); // 127.0.0.1
        ctx->user_port  = bpf_htons(SOCKS5_PORT); // 1080
    }
    return 1;
}
```

权限需求：`CAP_BPF`（远低于 `CAP_NET_ADMIN`）。覆盖静态二进制、Go 程序等所有可执行文件，不依赖 `LD_PRELOAD`。

### PRO 版 B：gVisor runsc（最强隔离）

Pod 使用 `runtimeClassName: gvisor`，workload 所有 syscall 经 sentry 拦截，网络调用走 gVisor netstack，天然无法绕过到 eth0。

```yaml
spec:
  runtimeClassName: gvisor
  containers:
  - name: workload
    env:
    - name: ALL_PROXY
      value: "socks5://127.0.0.1:1080"
```

隔离最彻底，但依赖节点安装 gVisor runsc runtime，且存在 ~5% syscall 兼容性风险。

### 方案对比

| 方案 | Workload 改造 | 权限需求 | 隔离强度 | 版本 |
|------|------------|---------|---------|------|
| 路由截断 + env 变量 | 需设置 proxy env | `NET_ADMIN`（initContainer） | 中 | Community |
| eBPF cgroup/connect | **无需改造** | `CAP_BPF` | 高 | PRO |
| gVisor runsc | env 变量（可选） | 节点安装 runsc | 最高 | PRO |

---

## sandbox 进程启动流程（增强后）

```
1. 生成 WireGuard 密钥对
2. 注册控制面 → 获取 overlay IP + Agent JWT
3. 初始化 shim.Netstack（localIP = overlayIP）
4. 初始化 EgressFilter（默认策略：allow overlay CIDR）
5. 启动 HTTP forward proxy（:1080）← 已实现
6. 启动 ForwardListener（监听 overlay IP，转发到 workload）← 新增
7. 启动 WireGuardManager（订阅 NetMap 变更）← 新增
8. 订阅 lattice.agent.{name}.policy → 热更新 EgressFilter ← 新增
9. 等待 SIGTERM
```

---

## CLI 新增参数

```
lattice-agent-sandbox start [flags]

新增 flags:
  --forward string     入站转发规则，格式 overlayPort:targetAddr
                       可重复指定，例: --forward 8080:127.0.0.1:8080
  --egress-allow string  允许的出站域名或 CIDR（逗号分隔）
  --egress-default-deny  启用白名单模式（默认放行 overlay 网段）
```

---

## 分阶段实现

| 阶段 | 内容 | 版本 |
|------|------|------|
| P1 | `shim.WireGuardManager` 动态 peer 同步 | Community |
| P1 | `shim.EgressFilter` 基础策略 | Community |
| P2 | `shim.ForwardListener` overlay 入站 | Community |
| P2 | NATS 策略热更新 | Community |
| P3 | Linux netns 路由截断 workload 隔离 | Community |
| P4 | eBPF cgroup/connect4 透明代理（无需 env 变量）| PRO |
| P4 | gVisor runsc workload 隔离 | PRO |

---

## 与现有代码的关系

| 现有组件 | 变化 |
|---------|------|
| `cmd/lattice-agent-sandbox/cmd/start.go` | 添加 `--forward`、`--egress-allow` 参数；启动 ForwardListener 和 WireGuardManager |
| `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` | 扩展 `sandboxCloser` 持有 ForwardListener 和 WireGuardManager；PRO 版加载 eBPF tproxy |
| `lattice-shim` | 新增 `shim/forward.go`、`shim/egress.go`、`shim/wgmanager.go` |
| `internal/agent/ebpf/sandbox_tproxy.bpf.c` | 新增（PRO）：cgroup/connect4 hook，透明重定向到 SOCKS5 proxy |
| HTTP/SOCKS5 proxy（`:1080`）| 保持不变，eBPF 透明代理的最终接收端 |

---

## 不在本设计范围内

- MCP Security Gateway：独立控制面组件，Q4 单独设计
- A2A 协议：overlay 通信的应用层协议，Q4 设计
- 审计批量上报控制面：当前 stderr 输出已满足近期需求
