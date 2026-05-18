# AI Agent 防护层次讨论

> 日期: 2026-05-18
> 性质: 头脑风暴记录（澄清现有能力边界与未来演进路线）

---

## 一、Lattice 对 AI Agent 做的两件事（易混淆）

Lattice 涉及 AI Agent 时有两条完全不同的路径，混在一起容易迷糊：

### 路径 A：AI 管理网络（MCP Server）— 零侵入

```
Claude Desktop / Cursor / 任意 AI 助手
    │  自然语言："把所有前端节点的 443 端口放开"
    │
    ▼
lattice-mcp (stdio MCP server)
    │  HTTP → /api/v1/ai/tools/call
    ▼
Lattice 控制面
    CheckToolAccess(RBAC) → writeToolSpan(trace)
    读操作直接执行，写操作需审批
```

- 不需写代码，在 Claude Desktop 配置里加 MCP server 即可
- AI 作为**网络管理员**，不是被保护的对象
- 已实现：14 个 MCP 工具（读直接执行、写需审批）

### 路径 B：Agent 的网络流量受保护（Sandbox）— 需要配合

```
AI Agent 进程
    │  HTTP_PROXY=http://127.0.0.1:1080  或  sb.DialContext()
    ▼
lattice sandbox start 进程
    ┌─────────────────────────────────┐
    │ gVisor pkg/tcpip（用户态协议栈）  │
    │   → EgressFilter (PRO)          │
    │   → TUNAdapter                  │
    │   → wireguard-go ChaCha20       │
    │   → ICE P2P / LRP relay         │
    └─────────────────────────────────┘
```

- 需要 AI 进程配合：设 `HTTP_PROXY` 或用 SDK 走 gVisor 的 netstack
- gVisor 是进程内运行，不需要容器
- 当前 E2E 部署形态是一个 K8s Pod，但代码架构上只是一个普通进程

### 当前防护漏洞

```
AI Agent 进程
    ├─→ 路径 1: HTTP_PROXY → gVisor → WireGuard   ✅ 加密隔离
    └─→ 路径 2: 直接 connect("eth0")               ❌ 不受控
```

gVisor `pkg/tcpip` 只拦截主动走它的流量，AI 进程直接连 eth0 就绕过去了。这不是 bug，是当前设计的局限——还没有在内核/系统调用层面强制拦截。

---

## 二、Agent 防护的三个层次

```
    当前实现          未实现              未实现
       │                 │                   │
  ─────▼─────      ──────▼──────      ───────▼───────
  │ 网络隔离  │  →  │  容器隔离   │  →  │  MicroVM  │
  │pkg/tcpip │      │  runsc     │      │Firecracker│
  ────────────      ──────────────      ──────────────

  对应 CRD 常量:
  SandboxGVisor      SandboxPod          SandboxMicroVM
```

| 层次 | 防护了什么 | 可类比 | CRD 常量 | 实现状态 |
|------|-----------|--------|----------|---------|
| 网络层 | AI 往外发的包走加密隧道，出站策略过滤 | 给进程配了个 VPN 网卡 | `SandboxGVisor` | ✅ 已实现 |
| 容器层 | 网络 + 文件系统隔离 + syscall 过滤 + /proc 阻断 | gVisor runsc（OCI 兼容）| `SandboxPod` | ❌ 只有常量，零代码 |
| MicroVM | 上面全部 + 硬件虚拟化隔离 | Firecracker，一人一 VM | `SandboxMicroVM` | ❌ 只有常量，零代码 |

### 网络隔离（当前 SandboxGVisor）

```
✅ 用户态 TCP/IP 协议栈（替代内核协议栈）
✅ wireguard-go 加密所有走它的流量
✅ 零特权（不需要 root、TUN、iptables）
✅ PRO: EgressFilter CIDR 出站过滤
❌ 文件系统隔离
❌ syscall 过滤（AI 进程的 syscall 照常走内核）
❌ 内存隔离
❌ /proc、/sys 等攻击面
```

### 容器隔离（SandboxPod，待实现）

在网络层的基础上新增：
- **文件系统**：gVisor sentry 接管 open/read/write，隔离 rootfs
- **syscall 过滤**：seccomp profile，只允许白名单 syscall
- **/proc 阻断**：AI 看不到宿主机的进程列表
- **资源限制**：cgroup CPU/内存绑定

类似：Docker，但内核是 gVisor 的 sentry 而非宿主内核。

### MicroVM 隔离（SandboxMicroVM，远期）

在容器层基础上新增硬件虚拟化：
- 每个 Agent 一个独立微型虚拟机
- 攻击面最小（只有 virtio 设备，无宿主内核攻击面）
- 类似 AWS Lambda/Fargate 的安全模型

---

## 三、实现状态汇总

### 已就绪

| 能力 | 版本 |
|------|------|
| gVisor `pkg/tcpip` 用户态网络栈 | Community + PRO |
| wireguard-go 加密 + ICE/LRP 连接 | Community + PRO |
| Zero-trust enrollment（一次性 token → JWT） | Community + PRO |
| 凭证持久化（重启免重注册） | Community + PRO |
| 工具调用 RBAC（CheckToolAccess + AllowedTools） | Community + PRO |
| tool_spans 可观测（每次工具调用记录 span） | Community + PRO |
| Sub-agent 委派（Delegate API，权限 ≤ 父级） | Community + PRO |
| MCP Server（14 工具，读执行/写审批） | Community + PRO |
| EgressFilter（CIDR 出站策略）+ ForwardListener + HTTP proxy | PRO |
| NATS 流量审计（flow_events，服务端已就绪） | PRO 服务端 |

### 待实现（Roadmap）

| 能力 | 等级 | 难度 |
|------|------|------|
| seccomp notify — 无侵入拦截 AI 进程 `connect()` | 🔜 短期 | 中 |
| eBPF `cgroup/connect4` — 强制 AI 流量走 WireGuard TUN | 🔜 短期 | 高（需 root + 5.10） |
| `natsAuditWriter` 沙箱侧接入（sandbox → NATS → flow_events） | 🔜 短期 | 低 |
| `SandboxPod` (gVisor runsc) — 容器级全隔离 | 中期 | 中 |
| `SandboxMicroVM` (Firecracker) — 硬件虚拟化隔离 | 远期 | 高 |
| `/calltree` API — 子 Agent 调用树查询 | 近期 | 低 |

---

## 四、对发版的建议

当前核心链路已跑通（E2E 通过），可发布 **v0.2.0（Agent Sandbox 首版）**：

**发版 blocker：**
- `natsAuditWriter` 沙箱侧接入（PRO flow_events 端到端打通）
- Release notes 说明当前局限（网络隔离而非全沙箱，AI 进程可绕过 gVisor 直连 eth0）

**可推迟到后续版本：**
- seccomp + eBPF PID/TUN 绑定
- `SandboxPod`（容器隔离）
- `SandboxMicroVM`（MicroVM 隔离）
- `lattice-sdk-python` 更新
- `/calltree` API
