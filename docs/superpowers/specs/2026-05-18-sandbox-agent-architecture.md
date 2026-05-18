# Sandbox Agent 架构参考

> 生成日期: 2026-05-18（基于 `cmd/lattice/cmd/sandbox/`、`internal/agent/gvisor/`）

## 概述

`lattice sandbox start` 是内置于主 `lattice` CLI 的零特权沙箱命令，社区版与 PRO 版均可用。它将 **gVisor 用户态网络栈**与 **Lattice WireGuard overlay** 融合，让 AI Agent 进程以普通用户权限运行，同时获得完整的 Lattice 网络身份（NATS 注册 + ICE/LRP 打洞 + LRP relay 回退），与基础设施节点的行为完全一致。

**社区版**（`//go:build !pro`）：gVisor 网络隔离 + 本地文件审计
**PRO 版**（`//go:build pro`）：新增出站策略过滤（EgressFilter）、入站端口转发（ForwardListener）、HTTP 正向代理

---

## 与普通节点的对比

| 维度 | 普通节点（`lattice up`） | Sandbox（`lattice sandbox start`） |
|------|------------------------|----------------------------------|
| 隔离方式 | 无（宿主机进程） | gVisor 用户态网络栈 |
| 特权需求 | root / `CAP_NET_ADMIN` | **零特权**（普通用户） |
| 网络栈 | 内核 TUN（`wf0`） | gVisor `pkg/tcpip` + TUNAdapter |
| WireGuard | 内核 `wgctrl` | `golang.zx2c4.com/wireguard`（用户态） |
| Provisioner | `KernelProvisioner`（iptables/eBPF） | `SandboxProvisioner`（无 iptables） |
| 注册方式 | HTTP 或 NATS | **NATS only**（`RegisterSandboxViaNATS`） |
| 凭证持久化 | 无 | JSON 文件（`/etc/lattice/sandbox-credentials.json`） |
| 审计日志 | eBPF ring buffer（PRO） | JSONL 文件（`/tmp/lattice-audit-<name>.jsonl`） |
| 出站策略 | eBPF TC ingress（PRO）/ iptables | `EgressFilter`（PRO sandbox only） |
| 入站转发 | 无 | `ForwardListener`（PRO） |
| HTTP 代理 | 无 | HTTP forward proxy（PRO） |
| ICE / LRP | ✅ 完整支持 | ✅ 完整支持（共享同一套基础设施） |

---

## 网络架构

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

## 代码文件结构

```
cmd/lattice/cmd/sandbox/
├── sandbox.go              # 公共命令定义（--name, --server-url, --token）
├── sandbox_shared.go       # 无 build tag — 共享工具（凭证读写、fileAuditWriter）
├── sandbox_community.go    # //go:build !pro — 社区版完整实现
└── sandbox_pro.go          # //go:build pro  — PRO 专属增强

internal/agent/gvisor/
├── sandbox.go              # gvisor.New() 入口，Config{ID, LocalIP, PolicyChecker, AuditWriter}
├── tun_adapter.go          # NewTUNAdapter：gVisor ↔ wireguard-go 的 packet 桥接
├── provisioner.go          # SandboxProvisioner（无 iptables，替换 KernelProvisioner）
└── shimfwd/
    ├── egress_filter.go    # EgressFilter（CIDR allowlist/denylist，实现 PolicyChecker）
    ├── forward_listener.go # ForwardListener（overlay 端口 → 宿主机地址转发）
    └── audit_writer.go     # AuditWriter 接口定义 + AuditEvent 结构
```

---

## 启动流程

### 社区版（`sandbox_community.go`）

```
1. 加载凭证 /etc/lattice/sandbox-credentials.json
   ├── 存在 → ResumeSandboxViaNATS(jwt, privKey)
   │   ├── OK  → 获取当前 overlay IP，跳过注册
   │   └── 失败（JWT 失效等）→ 降级走新注册
   └── 不存在 → 走新注册

2. 新注册：
   a. wgtypes.GeneratePrivateKey()
   b. infra.RegisterSandboxViaNATS(serverURL, token, name, pubKey)
      → 返回 Peer{Address, Token, LrpUrl}
   c. saveSandboxCredentials(privKey, peer.Token)  // 0600 权限

3. peer.LrpUrl != "" → agentconfig.Conf.EnableLrp = true

4. newFileAuditWriter("/tmp/lattice-audit-<name>.jsonl")

5. gvisor.New(Config{
       ID:            sandboxName,
       LocalIP:       localIP,
       AuditWriter:   auditWriter,
       PolicyChecker: nil,   // Community 放行全部出站
   })

6. gvisor.NewTUNAdapter(sb.Channel(), InjectIntoChannel(sb.Channel()))

7. agent.NewNode(ctx, NodeConfig{
       CustomTUN:   tunDev,
       CustomName:  sandboxName,
       CurrentPeer: currentPeer,   // 跳过 NATS 注册，直接用已获取的 Peer
       ProvisionerFactory: gvisor.NewSandboxProvisionerFactory(localIP, name),
   })

8. node.Start(ctx)
   go node.StartHeartbeat(ctx)        // 30s 心跳，维持控制面在线状态
   go 周期 RefreshConfig(15s)          // 兜底 NATS push 丢失

9. 阻塞等待 SIGINT/SIGTERM → node.Stop()
```

### PRO 专属（在社区版基础上增加）

```
4b. policyChecker := shimfwd.NewEgressFilter(egressPolicy)
    // egressPolicy 来自 --egress-allow CIDRs + --egress-default-deny

5b. gvisor.New(Config{
        ...,
        PolicyChecker: policyChecker,   // 启用出站过滤
        AuditWriter:   auditWriter,     // 文件（当前）/ NATS（待实现）
    })

8b. 可选：startHTTPProxy(ctx, sb, proxyAddr)    // --proxy-addr
    可选：ForwardListener 绑定 --forward 规则
```

---

## gVisor 组件详解

### TUNAdapter

`wireguard-go` 需要一个 `tun.Device` 接口来读写原始 IP 数据包。`TUNAdapter` 通过 channel 桥接 gVisor 的内部 packet 管道：

```
wireguard-go 加密报文
    → TUNAdapter.Write() → gVisor netstack 入口

gVisor netstack 出口（应用发出的明文包）
    → TUNAdapter.Read() → wireguard-go 加密 → 发送
```

### SandboxProvisioner

替代 `KernelProvisioner`，无任何 `ip link`/`ip addr`/iptables 系统调用：

- WireGuard peer 配置应用到 `wireguard-go` 用户态设备
- 路由管理通过 gVisor netstack 路由表（`netstack.AddRoute`）
- `SetupNAT` 是空操作（gVisor 内部处理 NAT）
- 完全无内核依赖，无需任何特权

### EgressFilter（PRO）

实现 `shimfwd.PolicyChecker` 接口，在 gVisor TCP/UDP `dial` 层拦截：

```go
type EgressFilter struct {
    allowCIDRs   []*net.IPNet
    defaultDeny  bool
}

func (f *EgressFilter) Check(dst net.IP, port uint16, proto string) error {
    if f.isAllowed(dst) {
        return nil
    }
    if f.defaultDeny {
        return fmt.Errorf("egress to %s:%d denied by policy", dst, port)
    }
    return nil
}
```

策略在连接建立前执行，拒绝操作发生在 gVisor 用户态内，不需要内核 netfilter。

### ForwardListener（PRO）

格式：`overlayPort:targetAddr`，例如 `--forward 8080:127.0.0.1:8080`

`ForwardListener` 在 gVisor netstack 内监听 `overlayIP:port`，接受连接后将流量转发到宿主机上的 `targetAddr`。这使其他 overlay peers 可以访问 sandbox 宿主机上运行的服务。

### HTTP Forward Proxy（PRO）

`--proxy-addr 127.0.0.1:1080` 启动一个 HTTP 正向代理，所有请求通过 `sb.DialContext()` 走 gVisor overlay（因此经过 WireGuard 加密）：

- **HTTP**：转发请求，去除 `Proxy-Connection` 头
- **HTTPS (CONNECT)**：Hijack 连接，建立 TCP 隧道

---

## 凭证持久化

```go
// sandbox_shared.go
type sandboxCredentials struct {
    PrivateKey string `json:"privateKey"`   // base64-encoded WireGuard private key
    JWT        string `json:"jwt"`          // Agent JWT（365 天有效）
}

// 路径：$LATTICE_CONFIG_DIR/sandbox-credentials.json
// 默认：/etc/lattice/sandbox-credentials.json
// 权限：0600
```

**为什么需要持久化**：容器每次重启都会丢失内存状态。没有持久化时，每次重启都需要消耗一次性的 enrollment token、生成新的 WireGuard 密钥，导致其他 peers 需要等待下一次 config push 才能识别新公钥。持久化后，重启只需调用 `ResumeSandboxViaNATS` 验证 JWT 即可恢复身份。

**Kubernetes 部署建议**：挂载 `emptyDir` 或 `PersistentVolumeClaim` 到 `/etc/lattice`，保证 Pod 重启期间凭证不丢失。

---

## 审计日志

### 社区版（本地文件）

```go
// sandbox_shared.go
type fileAuditWriter struct{ f *os.File }

func (w *fileAuditWriter) Write(event shimfwd.AuditEvent) error {
    line, _ := json.Marshal(event)
    _, err := fmt.Fprintf(w.f, "%s\n", line)
    return err
}
```

输出路径：`/tmp/lattice-audit-<sandboxName>.jsonl`

示例输出：
```json
{"identity":"my-agent","dst_ip":"10.100.0.2","dst_port":443,"protocol":"tcp","verdict":"allow"}
```

### PRO 版（当前：本地文件；规划：NATS 上报）

PRO sandbox 当前也使用 `fileAuditWriter`（路径可通过 `--audit-log` 自定义）。

服务端已就绪：`AuditConsumer`（`//go:build pro`）订阅 NATS `lattice.audit.flow`，写入 `la_flow_events` 表并通过 `traceId` 与 `la_tool_spans` 关联。

**待实现**：sandbox 侧 `natsAuditWriter`，将审计事件发布到 `lattice.audit.flow`。

---

## 心跳与配置刷新

```go
go node.StartHeartbeat(ctx)   // 每 30s 向控制面发送心跳
go func() {
    ticker := time.NewTicker(15 * time.Second)
    for {
        select {
        case <-ticker.C:
            node.RefreshConfig()  // 主动 pull 最新 NetworkMap
        case <-ctx.Done():
            return
        }
    }
}()
```

- **心跳**：保证 sandbox 在其他节点的 `ComputedPeers` 中保持 online，持续收到 NATS push
- **周期刷新**：兜底 NATS push 丢失场景（网络抖动、重连等）

---

## LRP Relay 回退

与普通节点行为完全相同，无额外配置：

```go
if currentPeer.LrpUrl != "" {
    agentconfig.Conf.EnableLrp = true
    agentconfig.Conf.RelayURL = currentPeer.LrpUrl
}
```

ICE 直连失败后，`agent.Node` 的状态机自动切换到 LRP relay（QUIC 优先，TCP 回退）。sandbox 的 LRP 路径不经过 gVisor netstack，直接在宿主机网络层处理。

---

## 命令行参考

```
lattice sandbox start [flags]

必填:
  --name         Sandbox 标识符（同时作为 AgentIdentity 名称和 LatticePeer 名称）
  --server-url   Lattice 控制面 URL（首次注册用）
  --token        Enrollment token（首次注册用，重启后自动从凭证文件恢复）

PRO 专属:
  --proxy-addr string       HTTP 正向代理监听地址（如 127.0.0.1:1080）
  --forward stringArray     入站端口转发规则（格式: overlayPort:targetAddr）
  --egress-allow string     允许出站的 CIDR 列表（逗号分隔）
  --egress-default-deny     启用出站白名单模式（不设则默认放行全部出站）
  --audit-log string        审计日志路径（默认 /tmp/lattice-audit-<name>.jsonl）
```
