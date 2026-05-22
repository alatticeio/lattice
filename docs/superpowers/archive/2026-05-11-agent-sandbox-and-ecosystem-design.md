# Lattice Agent 运行时沙箱与生态设计

> 状态: 设计阶段 | 关联: `2026-05-09-agent-isolation-wiring-design.md`

## 概述

当前 Lattice 实现了控制面（CRD、NATS 信令、Intent Engine）和数据面（WireGuard、ICE/STUN、LRP Relay），但设计文档中描述的"Agent 运行时沙箱"层（Socket 绑定、进程隔离、Sidecar 劫持、流量镜像审计）基本空白。本文档补全这些缺失模块的设计。

## 范围

5 个独立子项目，按依赖和价值分为 3 个阶段：

| 阶段 | 子项目 | 依赖 | 社区/PRO |
|------|--------|------|----------|
| Phase 1 | lattice-agent-sandbox + PID/TUN 绑定 + Sidecar 劫持 | 现有 Agent | 社区版 |
| Phase 2 | gVisor (runsc) 沙箱隔离 + eBPF 流量镜像审计 | Phase 1 | PRO |
| Phase 3 | 全局拓扑图可视化 + 个人玩家模式 | Phase 1 | 社区版 |

---

## Phase 1: Agent 运行时沙箱

### 1.1 `lattice-agent-sandbox` 二进制

新增 `cmd/lattice-agent-sandbox/`：

```
cmd/lattice-agent-sandbox/
├── main.go
└── cmd/
    ├── root.go          # 根命令
    ├── start.go         # 启动沙箱环境
    └── exec.go          # 在沙箱内执行 Agent 进程
```

**start 子命令**:

```
lattice-agent-sandbox start \
  --name my-agent \
  --mode cgroup           # cgroup | pod | microvm(pro)
  --network wf0           # 绑定的 TUN 设备名
  --policy-preset sandboxed
```

工作流：
1. 创建 cgroup（cpu, memory, io），限制 Agent 资源
2. 创建网络命名空间（可选隔离级别）
3. 将 Agent 进程 PID 写入 `cgroup.procs`
4. 调用 SandboxManager.AttachPID(pid, "wf0") 写入 eBPF map
5. 返回 `sandbox-id`，后续该进程所有流量受 eBPF policy 控制

**exec 子命令**:

```
lattice-agent-sandbox exec --sandbox-id <id> -- /usr/bin/python agent.py
```

在已创建的沙箱中启动进程。

### 1.2 Socket 级 PID → TUN 绑定

新增 eBPF 程序 `internal/agent/ebpf/sandbox_bind.bpf.c`：

```c
// 挂载点: cgroup/connect4, cgroup/sendmsg4
// 功能: 按 PID 强制 Agent 进程的 socket 走指定 TUN 设备

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, u32);     // PID
    __type(value, u32);   // ifindex of wf0
} pid_iface_map SEC(".maps");

SEC("cgroup/connect4")
int sandbox_connect(struct bpf_sock_addr *ctx) {
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u32 *ifindex = bpf_map_lookup_elem(&pid_iface_map, &pid);
    if (ifindex) {
        ctx->user_ifindex = *ifindex;
    }
    return 1;
}

SEC("cgroup/sendmsg4")
int sandbox_sendmsg(struct bpf_sock_addr *ctx) {
    // 同上逻辑，控制 UDP 发送
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u32 *ifindex = bpf_map_lookup_elem(&pid_iface_map, &pid);
    if (ifindex) {
        ctx->user_ifindex = *ifindex;
    }
    return 1;
}
```

Go 侧管理器 `internal/agent/sandbox/manager.go`：

```go
type SandboxManager struct {
    objs      *sandboxBindObjects   // bpf2go 生成的对象
    cgroupDir string                // cgroup 挂载点路径
}

func (m *SandboxManager) AttachPID(pid uint32, ifaceName string) error
func (m *SandboxManager) DetachPID(pid uint32) error
func (m *SandboxManager) ListBindings() (map[uint32]string, error)
```

### 1.3 Sidecar 模式拦截 Agent 外联意图

两种模式可选：

**模式 A (推荐，零侵入): seccomp notify + eBPF fast path**

```
Agent 进程发起 connect()
  → seccomp filter 捕获 (SECCOMP_RET_USER_NOTIF)
  → SeccompNotifier (Go 用户态) 收到通知
    → PolicyCache.Query(pid, dst_ip, dst_port)
      → 白名单命中: 返回 SECCOMP_USER_NOTIF_FLAG_CONTINUE，同时写入 eBPF allowlist map
      → 未命中: 返回 errno (EPERM)，触发 eBPF drop
```

新增 `internal/agent/sidecar/`：

```
internal/agent/sidecar/
├── notifier.go          # seccomp notify fd 监听
├── policy_cache.go      # eBPF map 的 Go 侧镜像，毫秒级查表
└── interceptor.go       # 统一拦截入口，结合 notifier + cache
```

**模式 B (轻量，兼容旧内核): LD_PRELOAD**

```
liblattice_intercept.so 注入 Agent 进程:
  connect() → 查询 PolicyCache → 允许/拒绝
  getaddrinfo() → DNS 过滤 → 只返回白名单域名
```

作为 seccomp 不可用时的 fallback。

### 1.4 数据流

```
┌─────────────────────────────────────────────────┐
│  lattice-agent-sandbox start                     │
│  ├─ cgroup: /sys/fs/cgroup/lattice/agent-123     │
│  ├─ netns: 共享宿主机（默认）或隔离                │
│  └─ eBPF: pid=12345 → ifindex=wf0                │
│                                                   │
│  lattice-agent-sandbox exec -- python agent.py    │
│  ┌──────────────────────────────────────┐         │
│  │  Agent 进程 (PID 12345)               │         │
│  │  connect("evil.com", 443)            │         │
│  └──────────────┬───────────────────────┘         │
│                 │                                  │
│                 ▼                                  │
│  ┌──────────────────────────────────────┐         │
│  │  seccomp notify (用户态)              │         │
│  │  PolicyCache.Query("evil.com")       │         │
│  │  → 未命中 → 返回 EPERM               │         │
│  └──────────────┬───────────────────────┘         │
│                 │ (白名单命中则放行)                 │
│                 ▼                                  │
│  ┌──────────────────────────────────────┐         │
│  │  eBPF tc_ingress on wf0               │         │
│  │  → 已有策略检查：源 IP + 端口 + 协议    │         │
│  └──────────────┬───────────────────────┘         │
│                 ▼                                  │
│         WireGuard 加密 → P2P/Relay 发送            │
└─────────────────────────────────────────────────┘
```

---

## Phase 2: 深度隔离与审计 (PRO)

### 2.1 gVisor (runsc) 沙箱

利用 gVisor 的 Sentry（用户态内核）实现 Agent 进程的系统调用级隔离。
gVisor 是 OCI runtime，与 Docker/K8s 原生兼容，不需要 KVM 硬件，随处可部署。

AgentIdentity CRD 扩展沙箱级别：

```yaml
# AgentIdentity.Sandbox 字段枚举扩展
sandbox:
  none      # Phase 1: cgroup + eBPF PID 绑定 (社区版)
  gVisor     # Phase 2: gVisor runsc 用户态内核隔离 (PRO)
  microVM    # 未来增强: Firecracker 内核级隔离 (PRO, 需要 KVM)
```

新增 `internal/agent/gvisor/` (build tag `pro`)：

```
internal/agent/gvisor/
├── runtime.go           # runsc 容器生命周期管理
├── netstack_bridge.go   # gVisor netstack → wf0 TUN 流量桥接
├── spec.go              # OCI spec 生成 (rootfs, 挂载, 网络配置)
└── community_stub.go    # 社区版 stub (build tag !pro, 返回 402)
```

**gVisor 网络模型 — 核心优势**：

gVisor 自带完整的 Go netstack（TCP/IP 协议栈），不需要 vsock 代理或 TAP 设备。
关键设计：**配置 Sentry 将 Agent 进程的 socket 操作直接交给宿主机 Agent 进程路由**。

```
gVisor Sandbox 内
┌─────────────────────────────────────────────────────────┐
│  Agent 进程 (Python/Node/Go)                            │
│  connect("api.internal", 443)                           │
└──────────────┬──────────────────────────────────────────┘
               │ [1] syscall (socket/connect/sendmsg)
               ▼
┌─────────────────────────────────────────────────────────┐
│  Sentry (gVisor 用户态内核)                              │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Go netstack (TCP/IP 协议栈)                      │  │
│  │  ├─ socket 拦截点 ─── PolicyCache 查表 ─────┐     │  │
│  │  ├─ 白名单命中 → 继续发包                    │     │  │
│  │  └─ 未命中 ──→ 返回 EACCES ────[审计]───────┤     │  │
│  └───────────────────────────────────────────────┘     │  │
│                                       │ or │ EACCES    │  │
│                                       ▼    ▼           │  │
│                          link endpoint   调用方收到错误 │  │
└──────────────────────────────┬──────────────────────────┘
                               │ [2] AF_PACKET raw socket
                               ▼
宿主机 Lattice Agent（gVisor 路径，零特权)
┌──────────────────────────────────────────────────────────┐
│  ┌─────────────────────────────────────────────┐         │
│  │  gVisor netstack bridge                     │         │
│  │  raw socket → IP 包重组 → wireguard-go      │         │
│  │  wireguard-go 直接附着在 netstack           │         │
│  │  link endpoint 上，无需 wf0 TUN              │         │
│  └───────────────────┬─────────────────────────┘         │
│                      │ [3] wireguard-go 加密             │
│                      │     (用户态 ChaCha20)              │
│                      ▼                                   │
│  ┌─────────────────────────────────────────────┐         │
│  │  UDP 封装 → 目标 Peer 查找 → ICE/LRP 路由   │         │
│  └───────────────────┬─────────────────────────┘         │
└──────────────────────┼──────────────────────────────────┘
                       │ [4] UDP :51820
                       ▼
┌──────────────────────────────────────────────────────────┐
│  FilteringUDPMux (单端口多路复用)                          │
│  ├─ STUN 包 → ICE mux (P2P 候选协商)                     │
│  └─ 非 STUN (WG 加密包) → DefaultBind → 直接发送          │
└──────────────────────┬───────────────────────────────────┘
                       │
          ┌────────────┴────────────┐
          ▼                         ▼
  ┌──────────────┐         ┌──────────────┐
  │  ICE P2P 直连 │         │  LRP Relay   │
  │  (UDP 洞)     │         │  (TCP/QUIC)  │
  └──────┬───────┘         └──────┬───────┘
         │                        │
         └────────┬───────────────┘
                  │ [5] 公网传输 (加密 WireGuard 载荷)
                  ▼
┌──────────────────────────────────────────────────────────┐
│  远端 Lattice Agent                                      │
│  ├─ UDP :51820 → wireguard-go 解密                      │
│  ├─ 若远端也是 gVisor: netstack deliver → Agent 进程     │
│  ├─ 若远端是内核路径: IP 包 → wf0 TUN → 内核路由 → 目标  │
│  └─ 目标收到 IP 包: src=10.100.0.x dst=10.100.0.y       │
└──────────────────────────────────────────────────────────┘
```

**图例（gVisor 端到端数据路径 5 步，零特权）**：

| 步骤 | 位置 | 动作 |
|------|------|------|
| [1] | gVisor Sandbox | Agent syscall 被 Sentry 截获，Go netstack 查 PolicyCache |
| [2] | gVisor → 宿主机 | 合法流量通过 AF_PACKET raw socket 到 netstack bridge |
| [3] | wireguard-go | 用户态 ChaCha20 加密，直接附着在 netstack link endpoint 上 |
| [4] | FilteringUDPMux | UDP 发送，STUN/WG 多路复用，分流 ICE / LRP |
| [5] | 公网 → 远端 | 加密载荷经 ICE P2P 或 LRP Relay 到达远端，对称解包 → 目标 |

**两种终局（Verdict Paths）**：

```
白名单命中 (allow):
  [1] Sentry netstack → 放行
  [2] AF_PACKET raw socket → netstack bridge
  [3] wireguard-go 加密 → link endpoint 发送
  [1a] ns.auditCh ← audit_event{verdict=allow} → AuditBatcher
  [4] FilteringUDPMux → [5] P2P/Relay → 远端解密 → 目标收到

未命中 (drop):
  [1] Sentry netstack → EACCES 返回调用方
  [1a] ns.auditCh ← audit_event{verdict=drop}
       → AuditBatcher → POST /api/v1/audit/batch → 控制面告警
```

> **注意**：图中不经过 wf0 TUN 和 eBPF TC hook。策略检查在 [1] Sentry 层完成，审计在 [1a]
> Go channel 完成。eBPF 只在内核路径（基础设施节点）使用，详见 2.1.1 节。

**启动流程**:

```
1. lattice-agent-sandbox start --mode gvisor --name agent-1
2. gvisor.Runtime.Start():
   a. 检查 runsc 二进制可用性 (PATH 中)
   b. 生成 OCI config.json:
      - root.path: Agent 工作目录 (bind mount)
      - process.args: [python, agent.py]
      - annotations:
          com.alattice.sandbox-id: agent-1
          com.alattice.wireflow.iface: wf0
   c. 调用 runsc --network=none run agent-1
     (--network=none 禁用 gVisor 默认 netstack，改用我们的 bridge)
   d. gVisor sentry 启动 → 走自定义 netstack bridge
   e. 向控制面注册 AgentIdentity (Sandbox=gvisor)
3. 返回 sandbox-id
```

**策略注入**：

利用 gVisor 的 netstack hook 机制：

```go
// netstack_bridge.go 核心逻辑

// LatticeNetstack 嵌入 gVisor netstack，在 socket 层注入策略检查
type LatticeNetstack struct {
    *gonet.Stack
    policyCache *sidecar.PolicyCache  // Phase 1 的缓存复用
}

func (ns *LatticeNetstack) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
    dstIP, dstPort := parseAddr(addr)

    if !ns.policyCache.Allow(sandboxPID, dstIP, dstPort) {
        // 违规尝试 — 记录审计事件
        ns.auditCh <- &AuditEvent{Verdict: "drop", DstIP: dstIP, DstPort: dstPort}
        return nil, fmt.Errorf("lattice: connection to %s denied by policy", addr)
    }

    // 合法流量 — 交给 wireguard-go 加密后经 link endpoint 发送
    // wireguard-go 直接附着在 gVisor netstack 的 link endpoint 上
    // 不依赖 wf0 TUN 设备，无需 root
    return ns.wgEndpoint.WritePacket(pkt)
}
```

**零特权架构 — 为什么不需要 TUN 设备**：

当前 Agent 已经在用 `wireguard-go`（用户态）处理 WG 逻辑，内核只负责创建 `wf0` TUN 设备作
为 wireguard-go 的附着点。gVisor netstack 提供了一个纯用户态的 link endpoint 接口，
可以完全替代 TUN：

```
当前（需 CAP_NET_ADMIN）:
  wireguard-go ──bind──→ wf0 TUN 设备 (ioctl SIOCSIFFLAGS, 需 root)
                          │
                          ├─ 入站: 内核 IP 栈 → TUN read → wireguard-go 解密
                          ├─ 出站: wireguard-go 加密 → TUN write → 内核 IP 栈
                          └─ eBPF TC hook 挂在 wf0 上做策略检查

gVisor 后（零特权，普通用户）:
  wireguard-go ──bind──→ gVisor netstack link endpoint (纯 Go interface)
                          │
                          ├─ 入站: raw socket → wireguard-go 解密 → netstack deliver
                          ├─ 出站: netstack route → wireguard-go 加密 → raw socket
                          └─ 策略检查 → Sentry socket 层 PolicyCache.Allow()
```

| 能力 | 当前 | gVisor 后 | 特权需求 |
|------|------|----------|----------|
| wireguard-go (加解密) | 用户态 | 用户态 (不变) | 无 |
| 网络协议栈 (TCP/IP) | 内核 (`wf0` TUN) | gVisor Go netstack | 无 |
| eBPF TC 策略 | `tc_ingress` hook on wf0 | Go 层 PolicyCache (等效替代) | 无 |
| TUN 设备创建 | `ioctl SIOCSIFFLAGS` | **不需要了** | **零** |
| ping / telnet / HTTP | 经 TUN → 内核 | 经 netstack 原生支持 | 无 |
| 启动命令 | `sudo lattice up` | `lattice-agent-sandbox start` | 普通用户 |

核心结论：**gVisor 后 Agent 沙箱完全零提权**。wireguard-go 切换到 gVisor link endpoint
作为附着点，所有网络在用户态闭环。`lattice-agent-sandbox` 作为普通用户进程运行。

**K8s 集成**：

```yaml
# 通过 RuntimeClass 将 Agent Pod 指定为 gVisor
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: lattice-gvisor
handler: runsc
---
# Agent Pod spec
spec:
  runtimeClassName: lattice-gvisor
  annotations:
    alattice.io/sandbox-mode: gvisor
    alattice.io/workspace-id: ws-xxx
```

**OCI 兼容性优势**：
- `docker run --runtime=runsc` 一键启动
- 镜像可直接用 Dockerfile 构建
- 不需要维护内核、rootfs 或 initramfs
- gVisor 已通过 Google GKE/Cloud Run 生产验证

### 2.1.1 策略执行分层 — eBPF 的定位

引入 gVisor 后 Lattice 的策略执行变为双轨分层：

```
┌─────────────────────────────────────────────────────────┐
│                    策略执行分层                           │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────────────┐                            │
│  │  AI Agent 工作负载       │  ← gVisor 沙箱            │
│  │  (Python/Node/Go 进程)   │                            │
│  │  ┌─────────────────────┐│                            │
│  │  │ Sentry Go netstack  ││  ← 策略: PolicyCache      │
│  │  │ (纯用户态)           ││     (Go 层 socket 拦截)   │
│  │  │ wireguard-go 加密    ││                            │
│  │  └─────────────────────┘│                            │
│  │  特权需求: 零            │                            │
│  │  eBPF: ❌ 不经过内核     │                            │
│  └─────────────────────────┘                            │
│                                                         │
│  ┌─────────────────────────┐                            │
│  │  基础设施节点             │  ← 内核 TUN 路径          │
│  │  (DB/Service/K8s 桥接)   │                            │
│  │  ┌─────────────────────┐│                            │
│  │  │ wf0 TUN 设备         ││  ← 策略: eBPF TC ingress  │
│  │  │ wireguard-go 加密    ││     (LPM Trie + Port Hash)│
│  │  └─────────────────────┘│                            │
│  │  特权需求: CAP_NET_ADMIN │                            │
│  │  eBPF: ✅ PRO 版高性能   │                            │
│  └─────────────────────────┘                            │
│                                                         │
│  ┌─────────────────────────┐                            │
│  │  社区版节点 (无 PRO)      │  ← 内核 TUN 路径          │
│  │  ┌─────────────────────┐│                            │
│  │  │ wf0 TUN 设备         ││  ← 策略: iptables/pfctl   │
│  │  │ wireguard-go 加密    ││     (传统防火墙)           │
│  │  └─────────────────────┘│                            │
│  └─────────────────────────┘                            │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

核心原则：

| 负载类型 | 隔离方式 | 策略执行 | 性能 | 特权 |
|----------|----------|----------|------|------|
| AI Agent 沙箱 | gVisor Sentry | Go PolicyCache (用户态) | 中等 (够用) | **零** |
| 基础设施 PRO | 内核 TUN + eBPF | TC ingress (LPM/Port) | 高 (内核态) | CAP_NET_ADMIN |
| 基础设施 社区版 | 内核 TUN | iptables/pfctl | 中等 | CAP_NET_ADMIN |

**eBPF 没有消失**——它仍然是 Lattice PRO 的核心差异化能力，只是从 Agent 沙箱里退场，
继续服务于需要极高性能策略的基础设施层（数据库集群东西向流量、K8s Service 网格、跨集群桥接）。

### 2.1.2 代码组织 — 独立 shim 库

gVisor netstack + wireguard-go 桥接层是**平台无关的纯网络代码**——不依赖 Lattice 的 CRD、NATS、控制面任何东西。拆为独立 repo 可以：
- 供其他项目单独引用（"给任意进程加零特权 WireGuard"）
- 独立版本和 CI，不拖主 repo 构建时间
- 接口层纯粹，边界清晰

**新 repo: `lattice-shim`**（独立 Go module，Apache 2.0）

```
lattice-shim/
├── shim/
│   ├── netstack.go           # gVisor netstack → wireguard-go link endpoint
│   ├── policy.go             # PolicyCache 接口（通用，调用方注入）
│   ├── audit.go              # audit event interface（通用）
│   └── wireguard.go          # wireguard-go 附着点封装
├── go.mod                    # module github.com/alatticeio/lattice-shim
├── go.sum
└── README.md
```

**核心接口（shim 只定义接口，Lattice 主 repo 实现）**:

```go
// shim/policy.go
type PolicyChecker interface {
    Allow(pid uint32, dstIP net.IP, dstPort uint16) bool
}

// shim/audit.go
type AuditWriter interface {
    Write(event AuditEvent) error
}

type AuditEvent struct {
    PID       uint32 `json:"pid"`
    SrcIP     string `json:"src_ip"`
    DstIP     string `json:"dst_ip"`
    DstPort   uint16 `json:"dst_port"`
    Protocol  string `json:"protocol"`
    Verdict   string `json:"verdict"` // "allow" | "drop"
}
```

**主 repo 改后结构**:

```
lattice/internal/agent/gvisor/       ← 依赖 lattice-shim
├── runtime.go           # runsc 生命周期管理
├── spec.go              # OCI spec 生成（Lattice 特定 config）
├── manager.go           # SandboxManager → CRD 注册、enrollment
├── policy_adapter.go    # sidecar.PolicyCache → shim.PolicyChecker
├── audit_adapter.go     # AuditBatcher → shim.AuditWriter
└── community_stub.go    # build tag !pro
```

**依赖关系**:

```
lattice-shim (零 Lattice 依赖)
  ├─ gvisor.dev/gvisor  (netstack)
  ├─ golang.zx2c4.com/wireguard
  └─ (无 Lattice 相关依赖)

lattice/internal/agent/gvisor/
  ├─ github.com/alatticeio/lattice-shim
  ├─ lattice/internal/agent/sidecar  (PolicyCache 实现)
  ├─ lattice/internal/agent/audit    (AuditBatcher 实现)
  └─ lattice/internal/agent/config   (CRD 类型)
```

### 2.2 未来增强：Firecracker MicroVM

当安全要求达到内核级隔离级别时（如执行用户提供的任意代码），启用 Firecracker 模式。
复用 `AgentIdentity.Sandbox: microvm` 字段。

```
部署环境判断:
  KVM 可用 && sandbox == microvm → Firecracker
  否则 && sandbox == gvisor → gVisor
  否则 → cgroup (Phase 1, 社区版)
```

Firecracker 方案保留为远期增强项，不在当前 Phase 2 实现范围内。详细设计参见本阶段附录。

### 2.3 流量审计 — 两条路径

gVisor 后审计数据来源有两条路径，按 Sandbox 级别选择：

**gVisor 路径（Sentry netstack 直接捕获）**：

代码中 `LatticeNetstack.DialContext()` 的 `ns.auditCh` 已覆盖 Sentry 层的 allow/drop 事件。
无需 eBPF——gVisor netstack 本身就能捕获每一个 socket 操作的 verdict。

**内核路径（Phase 1 cgroup 模式，保留 eBPF）**：

当 Agent 不走 gVisor（`sandbox: none`），继续沿用现有内核 TUN + eBPF 路径，此时需要 eBPF 镜像。

新增 `internal/agent/ebpf/tc_mirror.bpf.c` (build tag `pro`，仅内核路径使用)：

```c
// 挂载点: TC egress on wf0
// 功能: 按 Agent PID 过滤，镜像匹配的流量到 ring buffer

struct audit_event {
    u32 pid;
    u32 src_ip;
    u32 dst_ip;
    u16 dst_port;
    u8  protocol;
    u8  verdict;     // 0=allow, 1=drop
    u64 timestamp;
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);  // 256KB
} audit_events SEC(".maps");

// pid_filter_map: 需要审计的 Agent PID 集合
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 256);
    __type(key, u32);
    __type(value, u8);
} pid_filter_map SEC(".maps");

SEC("tc")
int tc_mirror(struct __sk_buff *skb) {
    // 解析 IP 头，检查 src_ip 是否属于被审计的 Agent
    // 匹配则构造 audit_event 写入 ring buffer
    // 始终返回 TC_ACT_OK (不阻断，只镜像)
}
```

Go 侧消费者 `internal/agent/audit/`：

```go
type AuditConsumer struct {
    reader    *ringbuf.Reader
    batcher   *AuditBatcher
    alerter   *AlertTrigger
}

// AuditBatcher: 缓冲 1000 条或 5 秒 flush 到控制面
// AlertTrigger: 检测异常模式（如 10 秒内 >100 次 drop）
```

审计数据流（双路径汇聚）：

```
gVisor 路径（进程级，零特权）:
  Sentry netstack DialContext() → ns.auditCh (Go channel)
    ──────────────────────────────────────────┐
                                              ├→ AuditBatcher → POST /api/v1/audit/batch
内核路径（eBPF，需要 CAP_BPF）:                  │     → t_audit_log (已有表，增加 traffic 类型)
  eBPF ringbuf → AuditConsumer.reader           │     → AlertTrigger (异常检测 → 告警通知)
    ──────────────────────────────────────────┘
```

### 2.3 与现有审计日志的集成

现有 `t_audit_log` 表（记录 API 写操作）扩展为支持流量审计事件：

```go
// models/audit_log.go 扩展
type AuditLog struct {
    // ... 现有字段 ...
    AuditType string  // "api" | "traffic" | "intent"  新增
    TrafficDetail *TrafficDetail `gorm:"serializer:json"`  // 新增
}

type TrafficDetail struct {
    PID       uint32 `json:"pid"`
    SrcIP     string `json:"src_ip"`
    DstIP     string `json:"dst_ip"`
    DstPort   uint16 `json:"dst_port"`
    Protocol  string `json:"protocol"`
    Verdict   string `json:"verdict"`  // "allow" | "drop"
}
```

---

## Phase 3: 可视化与生态

### 3.1 全局拓扑图

**后端** `internal/server/controller/topology_controller.go`:

```go
type TopologyController struct {
    peerCtrl     *PeerController
    relayCtrl    *RelayController
    presence     *nats.NodePresenceStore
    probeFactory *transport.ProbeFactory
}

type TopologyGraph struct {
    Nodes []TopoNode `json:"nodes"`
    Edges []TopoEdge `json:"edges"`
}

type TopoNode struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    IP        string `json:"ip"`
    Status    string `json:"status"`    // online | offline | pending
    TrafficRx uint64 `json:"traffic_rx"`
    TrafficTx uint64 `json:"traffic_tx"`
}

type TopoEdge struct {
    From    string `json:"from"`
    To      string `json:"to"`
    Type    string `json:"type"`       // p2p | relay
    Latency int64  `json:"latency_ms"`
}
```

API: `GET /api/v1/topology?workspaceId=`

前端 `fronted/src/pages/topology.vue`：使用 D3.js force layout，节点大小反映流量，连线类型区分 P2P/LRP，点击节点展示详情面板。

### 3.2 个人玩家模式

新增 `lattice quickstart` 子命令（`cmd/lattice/cmd/quickstart.go`）：

```
$ lattice quickstart
  → 检测环境（有无 K8s）
  → 无 K8s: 启动内嵌 latticed（单进程 SQLite）
  → 自动创建 workspace "personal"（默认 allow-all 策略）
  → 生成邀请链接: lattice join <token>
  → 输出: "Your personal mesh is ready! Dashboard: http://localhost:8080"

$ lattice join <token>
  → 解析 token 获取控制面地址和凭据
  → 自动配置 lattice.yaml
  → lattice up
  → 输出: "Connected! 3 peers online."
```

配置区分：`lattice.yaml` 新增 `mode` 字段：

```yaml
mode: personal  # personal | team | enterprise
```

个人模式下的差异：
- 默认策略：allow-all（而非 deny-all）
- 隐藏企业功能菜单
- 无审批工作流（auto_approve 全开）
- 无多租户（仅 personal workspace）

---

## 文件变更汇总

### 新增文件

```
# Phase 1
cmd/lattice-agent-sandbox/
cmd/lattice/cmd/quickstart.go
internal/agent/sandbox/manager.go
internal/agent/sidecar/notifier.go
internal/agent/sidecar/policy_cache.go
internal/agent/sidecar/interceptor.go
internal/agent/ebpf/sandbox_bind.bpf.c

# Phase 2 (PRO)
internal/agent/gvisor/runtime.go
internal/agent/gvisor/netstack_bridge.go
internal/agent/gvisor/spec.go
internal/agent/gvisor/community_stub.go
internal/agent/ebpf/tc_mirror.bpf.c
internal/agent/audit/consumer.go
internal/agent/audit/batcher.go

# Phase 3
internal/server/controller/topology_controller.go
fronted/src/pages/topology.vue
fronted/src/components/topology/
```

### 修改文件

```
internal/agent/node.go                # 集成 SandboxManager
internal/server/run.go                # 集成 TopologyController
internal/server/api.go                # 注册 /topology 路由
internal/server/models/audit_log.go   # 扩展 AuditType 和 TrafficDetail
cmd/lattice/cmd/root.go               # 注册 quickstart 子命令
fronted/src/layouts/default.vue       # 个人模式精简菜单
```

## 风险与应对

| 风险 | 应对 |
|------|------|
| seccomp notify 性能开销 | 白名单命中后走 eBPF fast path，seccomp 仅首次拦截 |
| gVisor syscall 兼容性 (~95%) | Agent SDK 文档标注兼容范围，不兼容的 syscall 走 Phase 1 cgroup fallback |
| gVisor 内存开销 (~25MB/Sandbox) | 单机百级 Agent 可接受，共享 Sentry 模式可优化 |
| eBPF ring buffer 丢事件 | 调大 buffer (512KB)，消费者独立 goroutine 不阻塞 |
| 个人模式与团队模式配置冲突 | `mode` 字段互斥，启动时校验 |
