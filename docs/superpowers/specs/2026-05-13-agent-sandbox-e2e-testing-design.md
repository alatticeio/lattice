# Agent Sandbox E2E 测试设计

> 状态: 设计阶段 | 关联: `2026-05-11-agent-sandbox-and-ecosystem-design.md`

## 概述

为 AI Agent sandbox（`lattice-agent-sandbox` + gVisor netstack）增加端到端测试，覆盖 Agent 注册、沙箱启动、策略执行、审计和吊销全链路。

## 架构

### 测试拓扑

```
k3d 集群
├── latticed (All-in-One 控制面)
│
├── Pod: companion-agent
│   Container 1: lattice agent (标准, WG tunnel)
│   Container 2: nginx (:8080)
│   注册方式: lattice up --token
│   角色: 连通性目标
│   VPN IP: 10.100.0.10
│
└── Pod: lattice-agent-sandbox
    Container: lattice-agent-sandbox start --wg
    注册方式: POST /api/v1/agent-isolation/register
    角色: 被测沙箱
    VPN IP: 10.100.0.20
```

### 数据流

```
场景 allow:
  execInPod(sandbox) → curl 10.100.0.10:8080
    → gVisor netstack DialContext
    → wireguard-go encrypt → UDP :51820
    → companion agent → decrypt → nginx
    → 断言: 200 OK

场景 deny:
  execInPod(sandbox) → curl 1.2.3.4:80
    → gVisor netstack DialContext
    → wireguard-go 查不到 peer → 丢包/timeout
    → 断言: 连接失败
```

## 前提条件

### --wg flag 实现

补齐 `--wg` flag 功能，将 wireguard-go 附着到 gVisor netstack。

**新增** `internal/agent/gvisor/wg_bridge.go` (`//go:build pro`)：
- 创建 wireguard-go UDP bind
- 桥接 gVisor netstack ↔ WireGuard:
  - 出站: `DialContext()` → wireguard-go encrypt → UDP bind → :51820 → Lattice overlay
  - 入站: UDP bind :51820 → decrypt → netstack deliver
- 复用 `sandbox.go` 中已有的 `pumpOutbound` 和 `Outbound` 回调

**修改** `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go`：
- `--wg` 启用时创建 WireGuardBind 传入 `gvisor.New(Config{WireGuardBind: bind})`

### 审计输出

`start_sandbox_pro.go` 的 AuditWriter 写本地 JSONL 文件 `/tmp/lattice-audit.jsonl`：

```jsonl
{"pid":12345,"dst_ip":"10.100.0.10","dst_port":8080,"protocol":"tcp","verdict":"allow"}
{"pid":12345,"dst_ip":"1.2.3.4","dst_port":80,"protocol":"tcp","verdict":"drop"}
```

e2e 测试通过 `execInPod cat /tmp/lattice-audit.jsonl` 读取并断言。

## 测试文件

### 新增 `test/e2e/agent_sandbox_test.go`

沿用现有 Ginkgo `Ordered` 模式，6 个场景。

**BeforeAll:**
1. `login(manageUrl)`
2. `createWorkspace("wf-e2e-sandbox-<ts>")`
3. `generateJoinToken(wsID)` ← 给 companion 用
4. `createEnrollmentToken(wsID, allowedTools)` ← 给 sandbox 用
5. `hostAliasesForNATS()`
6. `deployCompanionPod()` ← 标准 lattice agent + nginx
7. `waitForReady(companion)` → companion VPN IP
8. `deploySandboxPod()` ← lattice-agent-sandbox --wg
9. `waitForWGIP(sandbox)` → sandbox VPN IP
10. 创建 allow-all Policy

**测试场景:**

| # | It | 断言 |
|---|-----|------|
| 1 | `agent registration creates AgentIdentity` | AgentIdentity CRD exists, Phase=Active |
| 2 | `sandbox connects to companion via overlay` | `curl companion-ip:8080` → 200 OK |
| 3 | `sandbox traffic to non-lattice IP is blocked` | `curl 1.2.3.4:80` → timeout/non-200 |
| 4 | `policy deny blocks specific destination` | 更新 Policy 为 deny companion → `curl` timeout |
| 5 | `audit events captured` | `cat /tmp/lattice-audit.jsonl` 包含 allow/drop 事件 |
| 6 | `agent revoke stops sandbox connections` | DELETE /agents/:name → Phase=Revoked → 新连接失败 |

**AfterAll:** `cleanupWorkspace` + 清理 AgentIdentity + sandbox Pod

### 修改 `test/e2e/helpers_test.go`

新增辅助函数:
- `createEnrollmentToken(wsID, tools)` → token
- `registerSandboxAgent(serverURL, token, name, pubkey)` → JWT + VPN IP
- `deploySandboxPod(token, image)` → pod
- `revokeAgent(serverURL, name, namespace)`
- `readAuditLog(pod)` → []AuditEvent
- `execInSandbox(pod, cmd)` → output

## 实现范围

### 必须实现

| 变更 | 位置 |
|------|------|
| --wg flag 实现 | `internal/agent/gvisor/wg_bridge.go` (新) |
| --wg flag 启用 | `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` |
| Audit JSONL 输出 | `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` |
| e2e 测试文件 | `test/e2e/agent_sandbox_test.go` (新) |
| e2e 辅助函数 | `test/e2e/helpers_test.go` |
| PRO e2e sandbox 镜像 | CI / k3s Dockerfile |

### 不改

| 项 | 原因 |
|----|------|
| runsc / gVisor OCI runtime | gVisor netstack 编译进二进制，不需要 runsc |
| k3d 集群镜像 | 不需要预装额外工具 |
| 控制面审计 API / DB | 审计走本地文件 |
| PolicyChecker | 保持 nil（全放行），deny 靠"非 Lattice IP 无路由" |

## Sandbox Pod Spec

```yaml
containers:
- name: lattice-agent-sandbox
  image: ghcr.io/xxx/lattice-agent-sandbox:pro-xxx
  args: [start, --name=e2e-sandbox, --server-url=http://latticed:8080, --token=<token>, --wg]
  ports:
  - containerPort: 51820
    protocol: UDP
  # 不需要: privileged, CAP_NET_ADMIN, /lib/modules, --local-ip
```

companion Pod 复用现有 `deployAgentDeployment` 模式，加一个 nginx 容器在 8080。

## CI 集成

独立 workflow `sandbox-e2e.yml`，仅 PRO 构建时触发：

- **触发**: PR 带 `run-pro` label
- **构建**: `lattice-agent-sandbox` PRO 镜像 + `latticed` PRO 镜像 + companion agent PRO 镜像
- **集群**: k3d，和现有 e2e 使用相同模式
- **运行**: `go test ./test/e2e/... -run AgentSandbox`
- **独立**: 不依赖现有 e2e workflow，sandbox 测试不影响基础 e2e

---

## E2E 调试记录（2026-05-17）

在实际 CI 运行中逐一排查的问题与解决方案，记录于此供后续维护参考。

---

### 1. pion/ice nil pointer panic（companion 未收到 OFFER）

**现象：** sandbox 启动后不久 companion agent panic，栈帧指向 pion/ice 内部。
**根因：** sandbox 向 NATS 推送 OFFER 时，companion 的 ICEDialer `agent` 字段尚为 nil（或 `closed` 已置位），未做保护直接解引用。
**修复：** `internal/server/transport/ice_dialer.go` OFFER/ANSWER handler 增加提前返回保护：

```go
if i.closed.Load() {
    i.log.Debug("receive offer: dialer closed, dropping", "remoteId", remoteId)
    return nil
}
i.mu.Lock()
agent := i.agent
i.mu.Unlock()
if agent == nil {
    i.log.Debug("receive offer: agent nil, dropping", "remoteId", remoteId)
    return nil
}
```

---

### 2. 沙箱 crash loop（enrollment token 一次性消耗）

**现象：** sandbox Pod 重启后再次调用 register，server 返回 401/403，sandbox 陷入 crash loop。
**根因：** enrollment token 是一次性的，重启后重用同一 token 注册会被拒绝。
**修复：** `cmd/lattice/cmd/sandbox/sandbox_pro.go` 增加凭据持久化：

- 首次注册成功后将 `(privateKey, jwt)` 序列化到 `/etc/lattice/sandbox-credentials.json`（路径可通过 `LATTICE_CONFIG_DIR` 覆盖）。
- 重启时先尝试 `ResumeSandboxViaNATS`（只做 `GetNetMap`，不重新注册），失败才回退到 enrollment token 注册。
- 新增 `internal/agent/sandbox_register.go`，提取 `RegisterSandboxViaNATS` / `ResumeSandboxViaNATS` / `fetchNetMap` 三个函数。

---

### 3. `managementnats.NatsService` 类型名拼写错误

**现象：** `go build` 报 `undefined: managementnats.NatsService`。
**根因：** 实际类型名为 `NatsSignalService`，文档和代码中混用了简称。
**修复：** `internal/agent/sandbox_register.go:106` 改为 `*managementnats.NatsSignalService`。

---

### 4. E2E collectDiagnostics 未收集重启前容器日志

**现象：** sandbox crash loop 后 CI 仅打印当前容器日志，重启前的崩溃堆栈丢失。
**修复：** `test/e2e/e2e_test.go` `collectDiagnostics` 增加 `RestartCount > 0` 判断，对每个容器额外请求 `PodLogOptions{Previous: true}` 并打印。

---

### 5. companion nginx 监听端口 80 vs 8080

**现象：** sandbox 通过 HTTP proxy 请求 `companionVPNIP:8080` 返回 502。
**根因：** nginx 默认监听 80，但测试用的端口是 8080。
**修复：** `test/e2e/agent_sandbox_test.go` companion container 的 Command 增加 sed 替换：

```go
Command: []string{"sh", "-c",
    `sed -i 's/listen\s*80/listen 8080/g' /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'`,
},
```

---

### 6. 审计日志文件不存在

**现象：** Scenario 6 `cat /tmp/lattice-audit.jsonl` 报文件不存在。
**根因：** `gvisor.New` 调用时未传入 `AuditWriter`，shim 没有任何写文件的代码路径。
**修复：** `sandbox_pro.go` 增加 `fileAuditWriter` 实现，在 `gvisor.Config.AuditWriter` 注入：

```go
const auditLogPath = "/tmp/lattice-audit.jsonl"

type fileAuditWriter struct {
    mu sync.Mutex
    f  *os.File
}

func (w *fileAuditWriter) Write(event shimfwd.AuditEvent) error {
    data, _ := json.Marshal(event)
    w.mu.Lock()
    defer w.mu.Unlock()
    _, err := fmt.Fprintf(w.f, "%s\n", data)
    return err
}
```

---

### 7. 审计日志有 allow 事件但无 drop 事件

**现象：** Scenario 6 断言 `hasDrop=true` 失败；audit log 中只有 allow 事件，缺少 Scenario 5（DENY 策略）触发的 drop 事件。

**根因分析：**

1. Scenario 5 通过 sandbox HTTP proxy（`127.0.0.1:1080`）发送 `wget --timeout=3 http://companionVPNIP:8080`。
2. HTTP proxy 的 transport 使用 `sb.DialContext`（即 `gonet.DialContextTCP`）拨号。
3. DENY 策略生效后，companion 的 iptables 将 sandbox 发来的 TCP SYN DROP 掉，不返回任何响应。
4. `gonet.DialContextTCP` 等待 SYN-ACK，gVisor TCP 重传超时约 **127 秒**。
5. wget 的 `--timeout=3` 到期后客户端断开，但 Go HTTP server 不一定立即取消请求 context。
6. 在测试窗口内（30s Eventually + 10s Consistently），`gonet.DialContextTCP` 尚未超时，AfterDial 钩子未被调用，因此没有 drop 事件写入。

**修复：** `sandbox_pro.go` `startHTTPProxy` 给 DialContext 包装 5 秒超时：

```go
transport := &http.Transport{
    DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
        dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()
        return sb.DialContext(dialCtx, network, address)
    },
}
```

**效果：** TCP SYN 被 DROP 后，5 秒内 DialContext 以超时 error 返回 → AfterDial 钩子以 `err != nil` 被调用 → `verdict="drop"` 写入 audit log。Scenario 6 在测试窗口内可以观察到 drop 事件。

---

### 8. ICE WireGuard 握手始终超时（SYN 重传被误判为对端重启）

**现象：** BeforeAll 中等待 sandbox→companion WireGuard 握手的 `Eventually` 180 秒超时；companion 的 `lattice status` 显示 `Endpoint: (none)`，双方 WG handshake 从未完成。

**根因分析：**

ICE 握手的发起方（companion，initiator，peerID 更大）每 2 秒发一次 `HANDSHAKE_SYN`，持续 60 秒：

```go
ticker := time.NewTicker(2 * time.Second)
newCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
// 收到 ACK 后 i.cancel() 取消 ticker
```

非发起方（sandbox，non-initiator）收到第一个 SYN 时：
1. 发送 `HANDSHAKE_ACK`
2. 创建 pion ICE agent（`i.agent = agent`）
3. `agent.GatherCandidates()` → OnCandidate → 向 companion 发 OFFER

**2 秒后**，第二个 SYN 到达 sandbox 的 iceDialer：

```go
existingAgent := i.agent   // != nil（刚刚创建的 agent）
if existingAgent != nil {
    // 旧代码：误判为"对端重启"
    i.Close()  // closed=true, agent=nil, closeChan 关闭
    return nil
}
```

`i.Close()` 触发 `Dial()` 返回 `ErrDialerClosed` → `probe.restart()` → 新的 iceDialer（agent=nil）。

与此同时，companion 已收到第一个 ACK，取消了 SYN ticker，开始 GatherCandidates，向 sandbox 发送 OFFER（ICE candidate）。这些 OFFER 到达 sandbox 的新 dialer 时：

```go
agent := i.agent  // nil（新 dialer 刚建，未收到 SYN）
if agent == nil {
    // OFFER 被丢弃
    return nil
}
```

→ companion 的 ICE candidate 全部丢失 → `offerReady` 永远不会触发 → 双方各自等 65 秒超时 → WG 端点从未设置。

**误判原因分析：** 原代码注释称"agent 已存在 = 对端重启"，但该分支只在 `i.closed=false` 时可达。真正的对端重启 SYN 到达时，`i.closed=true`（ICE 成功后 500ms 的清理 goroutine 已调用 `i.Close()`），会走更早的 `if i.closed.Load()` 分支触发 `onRestart()`，根本不会进入 `existingAgent != nil` 的判断。因此 `i.closed=false` + `agent!=nil` 的组合**只可能**是 ICE 建立过程中的 SYN 重传，永远不是真正的对端重启。

**修复：** `internal/server/transport/ice_dialer.go`，将"SYN on active agent → Close"改为"重传 ACK，保留 agent"：

```go
if existingAgent != nil {
    // Dialer 仍开着（ICE 进行中），这是发起方的 SYN 重传，不是对端重启。
    // 重发 ACK 让发起方取消 ticker，继续 candidate 交换。
    // 关闭 dialer 会破坏正在进行的 ICE 建立流程。
    i.log.Debug("SYN retransmit during ICE setup, resending ACK", "remoteId", remoteId)
    _ = i.sendPacket(ctx, i.remoteId, grpc.PacketType_HANDSHAKE_ACK, nil)
    return nil
}
```

**效果：**
- 重传 SYN → 重发 ACK → companion 取消 ticker（幂等）→ ICE candidate 交换正常进行 ✓
- 第一个 ACK 丢失（如 sandbox 启动时 NATS 短暂断连）→ 后续 SYN 重传触发 resend ACK，最终 companion 收到 ✓
- 真正的对端重启：ICE 成功后 `i.Close()` 已将 `i.closed=true`，新 SYN 走 `i.closed` 分支触发 `onRestart()` ✓

---

*最后更新: 2026-05-17*
