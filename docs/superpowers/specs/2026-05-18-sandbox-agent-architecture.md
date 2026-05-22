# Sandbox Agent 架构参考

> 生成日期: 2026-05-21（基于 `cmd/lattice/cmd/sandbox/`、`internal/agent/gvisor/`、`internal/agent/runsc/`）

## 概述

`lattice sandbox start` 是内置于主 `lattice` CLI 的零特权沙箱命令，PRO 版可用。它支持两种隔离模式，让 AI Agent 进程以普通用户权限运行，同时获得完整的 Lattice 网络身份（NATS 注册 + ICE/LRP 打洞 + LRP relay 回退）。

| 模式 | 隔离层级 | 网络架构 | 适用场景 |
|------|---------|---------|---------|
| **pod 模式**（`--mode pod`） | 网络层（gVisor 用户态网络栈） | gVisor `pkg/tcpip` + TUNAdapter + wireguard-go（用户态） | 零特权环境，Agent 通过 SOCKS5 自觉接入 |
| **gvisor 模式**（`--mode gvisor`） | syscall 级（runsc 容器） | 两阶段：pod 内核 WireGuard + gVisor `--network=host` | 不受信任代码，syscall 强制拦截 |

---

## gVisor runsc 模式：两阶段架构

gVisor 的 `--network=host` 和 `--network=sandbox` 互斥——一个有 K8s 网络但无法创建 TUN，一个能创建 TUN 但无 eth0。解决方案是将工作拆成两个阶段：

```
Phase 1（pod 内核）:                Phase 2（runsc --network=host）:
┌──────────────────────────┐        ┌─────────────────────────────┐
│  bootstrapAgent()        │        │  runsc container             │
│                          │        │                             │
│  ① NATS 注册             │        │  PID 1: AI agent 二进制     │
│  ② wireguard-go → wg0    │        │  （直接 exec，无 shim）     │
│     (真实 /dev/net/tun)  │        │                             │
│  ③ 路由 + iptables       │        │  AI agent connect(peer)     │
│                          │        │    → gVisor sentry 拦截      │
│  node 持续存活 ──────────┼────────▶   → host kernel passthrough  │
│                          │        │    → pod 路由 → wg0 → overlay │
└──────────────────────────┘        └─────────────────────────────┘
```

**关键属性**：WireGuard 运行在真实内核上，不在 gVisor 内部。AI agent 通过 `--network=host` 继承 pod 的网络命名空间，其流量经 pod 路由进入 wg0 和 overlay。gVisor sentry 拦截所有 syscall 提供安全隔离，但网络不再依赖 gVisor 内部 netstack。

### 安全边界（gvisor 模式）

| 层级 | 机制 |
|------|------|
| Syscall 隔离 | gVisor sentry（所有 syscall 被拦截） |
| 网络访问 | Pod iptables/eBPF 规则作用在 wg0 |
| WireGuard 密钥 | 存在于 pod 内核，不在 gVisor 内 |
| CAP_NET_ADMIN | 不授予 gVisor 容器 |
| TUN 设备 | gVisor 内部不可用 |

---

## pod 模式：gVisor 用户态网络栈

pod 模式是目前最成熟的模式，在进程内嵌入 gVisor 用户态网络栈：

```
                    ┌─────────────────────────────┐
                    │       gVisor Sandbox         │
                    │                              │
  Agent 进程  ──▶   │  gVisor netstack (pkg/tcpip) │
  connect()         │        │                    │
                    │  [PRO] EgressFilter          │
                    │        │                    │
                    │  TUNAdapter (channel bridge) │
                    │        │                    │
                    │  wireguard-go Device         │
                    └──────────┬──────────────────┘
                               │ UDP :51820
                    ┌──────────▼──────────────────┐
                    │   FilteringUDPMux            │
                    │   STUN ──▶ ICE agent        │
                    │   non-STUN ──▶ WG DefaultBind│
                    └──────────┬──────────────────┘
                               │
              ┌────────────────┴──────────────┐
              │  ICE 打洞成功                  │  ICE 失败
              ▼                               ▼
        Direct P2P                    LRP relay (QUIC/TCP)
```

Sandbox 走的是与普通节点**完全相同**的信令路径：`NATS → ProbeFactory → ICE/LRP`。gVisor 只负责替换内核 TUN 设备，上层逻辑无感知。

---

## 三种模式对比

| 维度 | 普通节点（`lattice up`） | pod 模式 | gvisor 模式 |
|------|------------------------|----------|-------------|
| 隔离方式 | 无（宿主机进程） | gVisor netstack (in-process) | runsc 容器（syscall 拦截） |
| 特权需求 | root / `CAP_NET_ADMIN` | **零特权**（普通用户） | privileged（runsc 需要） |
| 网络栈 | 内核 TUN（`wf0`） | gVisor `pkg/tcpip` + TUNAdapter | 真实内核 TUN（pod kernel wg0） |
| WireGuard | 内核 `wgctrl` | wireguard-go（用户态） | wireguard-go（pod 内核） |
| Provisioner | `KernelProvisioner` | `SandboxProvisioner` | `KernelProvisioner`（pod iptables/eBPF） |
| 注册方式 | HTTP 或 NATS | **NATS only** | **NATS only** |
| 凭证持久化 | 无 | JSON 文件 | JSON 文件 |
| 出站策略 | eBPF TC / iptables | `EgressFilter`（PRO） | Pod iptables/eBPF |
| SOCKS5 代理 | 无 | 可选（--proxy-addr） | 无（直接路由） |
| 入站转发 | 无 | `ForwardListener`（PRO） | 无 |
| ICE / LRP | ✅ | ✅ | ✅ |

---

## 代码文件结构

```
cmd/lattice/cmd/sandbox/
├── sandbox.go              # 命令注册（--name, --server-url, --token, --mode）
├── sandbox_shared.go       # 共享工具（凭证读写、fileAuditWriter）
├── sandbox_community.go    # //go:build !pro — 社区版（pod 模式）
├── sandbox_pro.go          # //go:build pro  — PRO 入口 + 参数校验
├── sandbox_agent.go        # //go:build pro  — `lattice sandbox agent` 子命令（手动调试）
├── driver.go               # DriverConfig + IsolationDriver 接口
├── driver_pod.go           # //go:build pro  — PodDriver（进程内 gVisor netstack）
├── driver_runsc.go         # //go:build pro  — RunscDriver（两阶段 bootstrap + runsc）
└── sandbox_agent_register*.go  # agent 子命令注册

internal/agent/
├── gvisor/                 # 进程内 gVisor netstack（pod 模式）
│   ├── sandbox.go          # gvisor.New() 入口
│   ├── tun_adapter.go      # TUNAdapter：gVisor ↔ wireguard-go 桥接
│   └── provisioner.go      # SandboxProvisioner（无 iptables）
├── runsc/                  # runsc OCI 容器生命周期（gvisor 模式）
│   └── runsc.go            # Manager：OCI spec 生成 + 容器 start/stop
└── config/
    └── config.go           # SignalingURL, StunUrl, Port, WgPort 等字段
```

### 社区版 vs PRO 编译标签

```go
// sandbox_community.go
//go:build !pro

// sandbox_pro.go, sandbox_agent.go, driver_pod.go, driver_runsc.go
//go:build pro
```

社区版仅支持 pod 模式（不含 EgressFilter/ForwardListener/HTTP proxy）；gVisor runsc 模式为 PRO 独占。构建：`make EDITION=pro build`。

---

## 启动流程

### pod 模式（`sandbox_pro.go` → `PodDriver.Start()`）

```
1. 解析 egress 策略（CIDR allowlist/denylist）
2. 加载凭证 /etc/lattice/sandbox-credentials.json
   ├── 存在 → ResumeSandboxViaNATS(jwt, privKey)
   └── 不存在 → 走新注册
3. 新注册：wgtypes.GeneratePrivateKey() → RegisterSandboxViaNATS(...)
4. gvisor.New(Config{ID, LocalIP, AuditWriter, PolicyChecker})
5. gvisor.NewTUNAdapter(sb.Channel(), InjectIntoChannel)
6. agent.NewNode(ctx, NodeConfig{CustomTUN, CurrentPeer, ProvisionerFactory})
7. node.Start(ctx) + go node.StartHeartbeat(ctx)
8. 可选：Socks5Server + ForwardListener
9. 阻塞等待 SIGINT/SIGTERM → node.Stop()
```

### gvisor 模式（`sandbox_pro.go` → `RunscDriver.Start()`）

```
Phase 1 — bootstrapAgent(ctx) 在 pod 内核:
  与 pod 模式步骤 2-7 相同，但无需 CustomTUN 和 ProvisionerFactory
  （使用真实 /dev/net/tun，创建真实的 kernel wg0）

Phase 2 — runsc 容器:
  1. runsc.NewManager(Config{SandboxID, RootFS, AgentBinary, AgentArgs})
  2. mgr.Create()  → 写 OCI config.json（PID 1 = AgentBinary）
  3. mgr.Start(ctx) → runsc --network=host run <sandbox-id>
  4. 阻塞 select { ctx.Done() / mgr.Done() }
  5. defer node.Stop()（Phase 1 清理）
```

### gVisor 模式 CLI

```bash
lattice sandbox start \
  --mode gvisor \
  --name agent-001 \
  --server-url http://latticed:8080 \
  --token lt-xxx \
  --agent-rootfs /opt/lattice/agent-rootfs \
  --agent-binary /usr/local/bin/ai-agent \
  --agent-args --model,gpt-4 \
  --egress-allow 10.0.0.0/8
```

| Flag | 必填 | 描述 |
|------|------|------|
| `--mode gvisor` | 是 | 启用 gVisor runsc 隔离 |
| `--agent-rootfs` | 是 | 容器根文件系统路径 |
| `--agent-binary` | 是 | 容器内 AI agent 入口二进制 |
| `--agent-args` | 否 | AI agent 启动参数 |

---

## 凭证持久化

```go
// sandbox_shared.go
type sandboxCredentials struct {
    PrivateKey string `json:"privateKey"`   // base64-encoded WireGuard private key
    JWT        string `json:"jwt"`          // Agent JWT
}

// 路径：$LATTICE_CONFIG_DIR/sandbox-credentials.json
// 默认：/etc/lattice/sandbox-credentials.json
// 权限：0600
```

两种模式共享同一套凭证持久化机制。在 gVisor 模式下，凭证仅存在于 pod 内核（Phase 1），不会暴露给 gVisor 容器。

---

## 审计日志

### 当前（本地文件）

```go
type fileAuditWriter struct{ f *os.File }

func (w *fileAuditWriter) Write(event shimfwd.AuditEvent) error {
    line, _ := json.Marshal(event)
    _, err := fmt.Fprintf(w.f, "%s\n", line)
    return err
}
```

输出路径（pod 模式）：`/tmp/lattice-audit-<name>.jsonl`

### 未规划

- sandbox 侧 `natsAuditWriter`（将审计事件发布到 NATS `lattice.audit.flow`）。服务端 `AuditConsumer` 已就绪，但 sandbox 侧尚未实现。
- gVisor 模式审计（需设计如何在 gVisor sentry 与 pod 内核间传递审计事件）。

---

## 未规划（未来方向）

以下设计在讨论文档中提出，但**当前代码中未实现**：

- **eBPF/cgroup 内核级强制**（FAQ 中提到的"Phase 1"）——通过 `LD_PRELOAD` + cgroup + eBPF 在内核层强制所有 Agent 流量走 gVisor netstack，消除"Agent 不设代理即可绕过"的漏洞
- **eBPF TC filter on wf0** —— 将 eBPF 策略附加到 wf0 TUN 接口，在内核层过滤
- **seccomp notify** —— 通过 seccomp 用户态通知机制精确控制 Agent 的 socket 调用
- **Sidecar 劫持** —— 在 K8s Pod 内通过 iptables 劫持所有 Agent 出站流量

参见 `docs/faq/ebpf-sandbox.md` 和 `docs/superpowers/adr/0001-gvisor-library-vs-runsc.md`。

---

## 命令行参考

```
lattice sandbox start [flags]

必填:
  --name            string   Sandbox 标识符
  --server-url      string   Lattice 控制面 URL
  --token           string   Enrollment token

可选:
  --mode            string   隔离模式：pod（默认）| gvisor（PRO）
  --ready-wait      duration WireGuard 就绪等待（agent 子命令，默认 3s）

pod 模式（PRO）:
  --proxy-addr      string   SOCKS5 代理监听地址（如 127.0.0.1:1080）
  --forward         strings  入站转发规则（格式: overlayPort:targetAddr）
  --egress-allow    string   允许出站的 CIDR 列表（逗号分隔）
  --egress-default-deny      启用出站白名单模式

gvisor 模式（PRO）:
  --agent-rootfs    string   容器根文件系统路径
  --agent-binary    string   AI agent 入口二进制
  --agent-args      strings  AI agent 启动参数
```
