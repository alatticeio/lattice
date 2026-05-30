# Sandbox Agent 架构参考

> 生成日期: 2026-05-21（基于 `cmd/lattice/cmd/sandbox/`、`internal/agent/gvisor/`、`internal/agent/runsc/`）
> 更新日期: 2026-05-26（新增 `sandbox run` 命令设计）

## 概述

Lattice Sandbox 为 AI Agent 提供隔离的网络身份，让 Agent 进程以普通用户权限运行，同时获得完整的 Lattice 网络（NATS 注册 + ICE/LRP 打洞 + LRP relay 回退）。

**主要用户命令**（PRO 版可用）：

| 命令 | 定位 | 描述 |
|------|------|------|
| `lattice sandbox run` | **主命令（用户首选）** | 一键启动沙箱 + 注入代理 + 执行 AI Agent |
| `lattice sandbox start` | 底层命令（进阶调试） | 仅启动沙箱守护进程，不执行 Agent |

**隔离模式**：

| 模式 | 隔离层级 | 网络架构 | 适用场景 |
|------|---------|---------|---------|
| **pod 模式**（`--mode pod`，默认） | 网络层（gVisor 用户态网络栈） | gVisor `pkg/tcpip` + TUNAdapter + wireguard-go（用户态） | 零特权环境，通过 SOCKS5 代理接入 |
| **gvisor 模式**（`--mode gvisor`） | syscall 级（runsc 容器） | 两阶段：pod 内核 WireGuard + gVisor `--network=host` | 不受信任代码，syscall 强制拦截 |

---

## `lattice sandbox run`（主用户命令）

### 设计目标

将"启动沙箱 + 配置代理 + 运行 Agent"三个步骤合并为**一条命令**。用户不需要了解 SOCKS5 端口、代理配置、进程管理等细节。

### 使用示例

```bash
# 最简用法
lattice sandbox run \
  --name my-agent \
  --server-url http://latticed:8080 \
  --token lt-xxx \
  -- python agent.py --task "analyze data"

# 指定代理端口（默认随机）
lattice sandbox run \
  --name my-agent \
  --server-url http://latticed:8080 \
  --token lt-xxx \
  --proxy-addr 127.0.0.1:1080 \
  -- claude --model claude-opus-4-6

# 指定出站策略
lattice sandbox run \
  --name my-agent \
  --server-url http://latticed:8080 \
  --token lt-xxx \
  --egress-allow 10.0.0.0/8 \
  -- python agent.py
```

### 执行流程

```
lattice sandbox run -- <ai-agent> [args...]
         │
         ▼
1. 启动 Sandbox（pod 模式）
   ├── 注册/恢复凭证
   ├── 初始化 gVisor netstack
   └── 等待 WireGuard 就绪（≤ 3s）
         │
         ▼
2. 启动 SOCKS5 代理
   ├── 监听 127.0.0.1:<随机端口>（默认 :0，OS 分配）
   └── 获取实际绑定的端口号
         │
         ▼
3. 构造子进程环境
   ├── 继承当前进程的所有环境变量
   ├── 注入 ALL_PROXY=socks5://127.0.0.1:<port>
   ├── 注入 all_proxy=socks5://127.0.0.1:<port>（小写兼容）
   └── 注入 LATTICE_SANDBOX_NAME=<name>（可选元数据）
         │
         ▼
4. exec AI Agent 子进程
   └── os/exec.Cmd{Env: injectedEnv, Stdout: os.Stdout, Stderr: os.Stderr}
         │
         ▼
5. 等待子进程退出
   └── 子进程退出（无论成功/失败）→ sandbox 自动清理 → 进程退出
```

### 代理注入：近零入侵原则

`ALL_PROXY` / `all_proxy` 是业界标准环境变量，被以下工具自动识别：

| AI Agent / 工具 | 识别 SOCKS5 代理 |
|-----------------|----------------|
| curl、wget | ✅ `ALL_PROXY` |
| Python `requests` | ✅（通过 urllib3） |
| Python `httpx` | ✅ |
| Node.js（undici/fetch） | ✅ `ALL_PROXY` |
| Go 标准库 `net/http` | ✅ `ALL_PROXY` |
| Claude CLI | ✅ |
| OpenAI SDK（Python/Node） | ✅ |

**AI Agent 无需修改任何代码**，只需通过标准 HTTP/HTTPS 客户端发起请求，流量自动路由到 Lattice overlay 网络。

### 生命周期绑定

```
AI Agent 进程  ──退出──▶  sandbox run 检测退出
                               │
                               ▼
                         sandbox.Stop()
                         node.Stop()
                         SOCKS5 服务关闭
                         进程退出（透传 exit code）
```

- AI Agent 是主体：Agent 退出，Sandbox 跟随退出
- 反向也成立：Sandbox 内部错误（如 NATS 连接中断）→ SIGTERM 子进程 → 等待 5s 后 SIGKILL

### Flag 设计

```
lattice sandbox run [flags] -- <command> [args...]

必填:
  --name            string   Sandbox 标识符
  --server-url      string   Lattice 控制面 URL
  --token           string   Enrollment token

可选:
  --mode            string   隔离模式：pod（默认）| gvisor（PRO）
  --proxy-addr      string   SOCKS5 代理监听地址（默认 127.0.0.1:0，随机端口）
  --egress-allow    string   允许出站的 CIDR 列表（逗号分隔）
  --egress-default-deny      启用出站白名单模式
  --forward         strings  入站转发规则（格式: overlayPort:targetAddr）
  --ready-timeout   duration WireGuard 就绪超时（默认 10s）
```

`--` 之后的所有内容作为子命令传递，`sandbox run` 本身不解析。

### 与 `sandbox start` 的区别

| 维度 | `sandbox run` | `sandbox start` |
|------|--------------|----------------|
| 定位 | 主用户命令 | 底层命令（进阶/调试） |
| Agent 执行 | ✅ 自动 exec AI Agent | ❌ 仅启动沙箱守护进程 |
| 代理注入 | ✅ 自动注入 ALL_PROXY | ❌ 需手动设置 |
| 生命周期 | Agent 进程控制 | 需手动 SIGTERM |
| 典型使用者 | 最终用户 / CI/CD | 调试、sidecar 场景 |

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
  (ALL_PROXY)       │        │                    │
  SOCKS5 Client     │  [PRO] EgressFilter          │
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
| SOCKS5 代理 | 无 | ✅（`sandbox run` 自动注入） | 无（直接路由） |
| 入站转发 | 无 | `ForwardListener`（PRO） | 无 |
| ICE / LRP | ✅ | ✅ | ✅ |

---

## 代码文件结构

```
cmd/lattice/cmd/sandbox/
├── sandbox.go              # 命令注册（start + run 子命令）
├── sandbox_run.go          # `sandbox run` 实现：exec AI Agent + ALL_PROXY 注入
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

### `sandbox run` 流程（`sandbox_run.go`）

```
1. 解析 -- 之后的命令行作为子进程命令
2. 调用 driver.Start(ctx)（与 sandbox start 相同的沙箱启动逻辑）
3. 等待 WireGuard 就绪信号（driver.Ready() channel，超时 --ready-timeout）
4. 获取 SOCKS5 实际监听地址（driver.ProxyAddr()）
5. 构造注入环境：
   env = os.Environ()
   env = append(env, "ALL_PROXY=socks5://"+proxyAddr)
   env = append(env, "all_proxy=socks5://"+proxyAddr)
   env = append(env, "LATTICE_SANDBOX_NAME="+name)
6. cmd = exec.CommandContext(ctx, args[0], args[1:]...)
   cmd.Env = env
   cmd.Stdin/Stdout/Stderr = os.Stdin/Stdout/Stderr
7. cmd.Start()
8. go 监听 cmd.Wait() → exitCode
9. select:
   - cmd.Wait() 完成 → driver.Stop() → os.Exit(exitCode)
   - ctx.Done() → cmd.Process.Signal(SIGTERM) → 5s → SIGKILL → driver.Stop()
```

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

- **eBPF cgroup_sock_addr 透明代理**：通过 `BPF_PROG_TYPE_CGROUP_SOCK_ADDR` 在内核层将 AI Agent 的所有出站连接重定向到 SOCKS5 代理，无需 Agent 感知 `ALL_PROXY`。适用于 Agent 不识别 `ALL_PROXY` 的场景（如自定义网络库）。需要 root 权限 + Linux 5.7+。
- **eBPF sockops**（`BPF_PROG_TYPE_CGROUP_SOCK_ADDR`）——通过 cgroup eBPF 在内核层强制所有 Agent 流量走 gVisor netstack，消除"Agent 不设代理即可绕过"的漏洞
- **eBPF TC filter on wf0** —— 将 eBPF 策略附加到 wf0 TUN 接口，在内核层过滤
- **seccomp notify** —— 通过 seccomp 用户态通知机制精确控制 Agent 的 socket 调用
- **Sidecar 劫持** —— 在 K8s Pod 内通过 iptables 劫持所有 Agent 出站流量

参见 `docs/faq/ebpf-sandbox.md` 和 `docs/superpowers/adr/0001-gvisor-library-vs-runsc.md`。

---

## 命令行参考

### `lattice sandbox run`（推荐）

```
lattice sandbox run [flags] -- <command> [args...]

必填:
  --name            string   Sandbox 标识符
  --server-url      string   Lattice 控制面 URL
  --token           string   Enrollment token

可选:
  --mode            string   隔离模式：pod（默认）| gvisor（PRO）
  --proxy-addr      string   SOCKS5 代理监听地址（默认 127.0.0.1:0，随机端口）
  --egress-allow    string   允许出站的 CIDR 列表（逗号分隔）
  --egress-default-deny      启用出站白名单模式
  --forward         strings  入站转发规则（格式: overlayPort:targetAddr）
  --ready-timeout   duration WireGuard 就绪超时（默认 10s）

注: -- 后的所有内容作为 AI Agent 子命令执行
```

### `lattice sandbox start`（底层）

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
