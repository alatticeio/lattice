# Sandbox Agent 统一注册设计讨论

> 记录 2026-05-17 对 agent sandbox 架构的分析与重构决策。

---

## 背景：E2E 测试失败分析

### 失败现象

`Agent Sandbox` E2E 测试超时（180s），companion pod 持续报：

```
Failed to send handshake initiation: no known endpoint for peer
```

sandbox pod 日志在 "Sandbox ready" 之后没有任何配置更新记录，WireGuard 握手从未建立。

### 根本原因链

```
sandbox 注册完成
  → node.Start() 调用 GetNetworkMap
  → 初始 config: ComputedPeers=[] (controller 还未 reconcile，companion 未加入)
  → applyRemotePeers 空跑，companion 永远没有被 AddPeer
  → sandbox 侧不存在 companion 的 WG peer entry
  → sandbox 侧没有 ICE probe，不发 SYN
  → companion 侧加了 sandbox 为 static WG peer（等待握手）
  → WireGuard 尝试给 sandbox 发握手 → "no known endpoint"（无限循环）
```

### 次要问题：messageHandler nil 竞态

在 `node.go` 的 `NewNode` 中，NATS 订阅在 Phase 2 建立，但 `messageHandler` 在 Phase 3 才赋值：

```go
// Phase 2: 订阅建立
natsSignalService.Subscribe("lattice.signals.peers.xxx", node.probeFactory.Handle)

// Phase 3: messageHandler 才赋值
node.messageHandler = NewMessageHandler(...)
```

`probeFactory.Handle` 内部的 `GetOnMessage` 闭包：

```go
GetOnMessage: func() func(...) error {
    if node.messageHandler == nil {
        return nil  // ← 若 NATS push 在 Phase 3 完成前到达，消息被静默丢弃
    }
    return node.messageHandler.HandleEvent
},
```

若 controller 的配置 push 在 Phase 2~3 之间到达（约 2 秒窗口），整条更新被丢弃，sandbox 永远拿不到 companion peer。

### tunAdapter.Read() 忙轮询

```go
func (t *tunAdapter) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
    pkt := t.ch.Read()  // gVisor channel.Read() 非阻塞
    if pkt == nil {
        return 0, nil   // wireguard-go TUN 读 goroutine 以 100% CPU 自旋
    }
}
```

不是根本原因，但会在单核容器里抢占 ICE/NATS goroutine 调度时间。

---

## 架构分析：为什么 sandbox 没有用普通 agent 的流程

### 普通 agent 的 NATS register

```go
// client.go:Register
registryRequest := &dto.PeerDto{
    AppID:   config.Conf.AppId,
    Platform: runtime.GOOS,
    Token:   token,
    // ← 没有 PublicKey
}
// NATS → "lattice.signals.peer" register
// 服务端生成 WireGuard 密钥对，响应包含 PrivateKey
```

**私钥由服务端生成、下发。**

### sandbox 用 HTTP 注册的原因

**唯一本质区别：私钥不能由服务端生成。**

```go
// start.go
privKey, _ := wgtypes.GeneratePrivateKey()  // sandbox 本地生成
pubKey := privKey.PublicKey().String()
// HTTP 注册时只发 pubKey，privKey 永远不离开 sandbox
```

这是安全模型要求：sandbox 运行不受信任的代码（AI agent），私钥必须在进程内生成，服务端无法知道。

### 这一差异导致的额外复杂度

为了传 `publicKey`，必须走 HTTP（NATS register 的 PeerDto 里原本没有 PublicKey 字段被使用）：

```
HTTP register (enrollmentToken + publicKey) → JWT
    ↓
fetchPeerViaNATS → GetNetMap (等 IP，ComputedPeers 可能为空)
    ↓
NewNode(CurrentPeer: ...) → 绕过 NATS register
    ↓
node.Start() → GetNetMap 再拿一次（还是可能为空）
    ↓
等 NATS push ... 15s 刷新 ... 超时
```

**所有的复杂性、所有的竞态，都源于 HTTP 这条绕路。**

---

## 重构决策

### 核心方向：sandbox 通过 NATS register 完成注册

`PeerDto` 里已有 `PublicKey` 字段，只是未在 register 中使用。改动：

1. **Server**：`Register` NATS handler 检测到 `publicKey` 非空时，走 enrollment token 路径（调用 `agentRegistrationService.RegisterAgent()`），返回包含 JWT 的 `*infra.Peer`（`Token` 字段 = agentJWT，`PrivateKey` 为空）。

2. **Client**：`NodeConfig` 增加 `PublicKey`、`PrivateKey` 字段；`ctrClient.Register()` 带上 publicKey；server 返回空 PrivateKey 时，使用本地生成的 PrivateKey 填充。

3. **start.go**：删除 `registerWithServer`（HTTP）和独立的 `fetchPeerViaNATS`，通过 `agent.RegisterSandboxViaNATS()` 完成注册和 IP 等待（全程 NATS）。

4. **Node.go**：将 `messageHandler` 初始化提前到 NATS Subscribe 之前，修复竞态。

### 为什么保留 CurrentPeer bypass

gVisor netstack 在 `New(cfg)` 时需要 `LocalIP` 来配置虚拟 NIC。这个 IP 来自 controller 的 IPAM，在注册后才分配。因此：

- 注册（NATS register + 等 IP）必须在 `createSandbox` 之前完成
- `createSandbox` 完成后，`NewNode` 通过 `CurrentPeer` 传入已知身份，跳过二次注册

这是 gVisor 架构约束，而非设计缺陷。

### 是否需要独立的 lattice-agent-sandbox binary

**不需要**。改造后 sandbox 启动逻辑大幅简化：

| 方面 | 改造前 | 改造后 |
|---|---|---|
| 注册 | HTTP + NATS 两步 | NATS 一步 |
| 特殊逻辑 | registerWithServer, fetchPeerViaNATS, discoverNATSURL | 无（合入 RegisterSandboxViaNATS） |
| 与普通 agent 差异 | 注册机制、TUN 设备、provisioner | 只剩 TUN 设备、provisioner |

gVisor 相关代码已在 `//go:build pro` 后面，可直接条件编译到主 binary。`lattice sandbox start` 比独立 binary 更符合 CLI 惯例，也减少了维护负担。

### addStaticPeer 设计说明

companion 通过 `addStaticPeer` 添加 sandbox 为静态 WG peer（无 endpoint），等待 sandbox 主动发起握手。sandbox 通过 ICE 找到 companion 的 UDP endpoint 后，发起 WG 握手，companion 从握手包的源地址学到 sandbox 的 endpoint。

这是有意为之的非对称设计：companion 不需要知道 sandbox 的 endpoint，只需接受握手。修复注册时序后，sandbox 可以正常走 ICE → WG 握手流程。

---

## 改造后的 sandbox 启动流程

```
lattice sandbox start --token <enrollment-token> --server-url <url>
    ↓
generateLocalWireGuardKeyPair()
    ↓
agent.RegisterSandboxViaNATS(ctx, serverURL, enrollmentToken, sandboxName, privKey)
    → NATS register (enrollmentToken + publicKey) → server 创建 LatticePeer + AgentIdentity → 返回 JWT
    → NATS GetNetMap (JWT) → 轮询等待 IP 分配 → 返回 *infra.Peer
    ↓
createSandbox(sandboxName, localIP, egressPolicy)  // gVisor netstack
    ↓
agent.NewNode(ctx, &NodeConfig{
    CurrentPeer:        currentPeer,  // 已知身份，跳过 NATS register
    CustomTUN:          tunDev,       // gVisor TUN adapter
    ProvisionerFactory: sandboxProvisioner,
})
    ↓
node.Start(ctx)  // GetNetMap → ApplyFullConfig → AddPeer(companion) → ICE → WG handshake
```

与普通 agent 唯一的运行时差异：gVisor 替代 iptables/eBPF 做策略执行。

---

## E2E Scenario 7 失败分析：agent revocation 后 AgentIdentity "not found"

### 失败现象

Scenario 7 `agent revocation stops sandbox connections` 失败；`Eventually` 在 30 秒内始终返回：

```
AgentIdentity not found: agentidentities.alattice.io "sandbox-xxx" not found
```

### 根本原因

`RevokeAgent`（`internal/server/service/agent_registration.go`）直接调用 `k8s.Delete()` 删除了 `AgentIdentity` CRD：

```go
// 旧代码：删除资源
if err := s.k8s.Delete(ctx, identity); k8sclient.IgnoreNotFound(err) != nil {
    return fmt.Errorf("delete AgentIdentity: %w", err)
}
```

但 Scenario 7 的期望是：**CRD 保留，且 `Status.Phase = Revoked`**。

这与整体架构不一致——`AgentIdentityReconciler` 将 Revoked/Expired 设计为终态 phase 字段，资源本身用于审计追踪；`CheckToolAccess` 也是通过读取 phase 来拦截已撤销 identity 的后续调用，资源被删除后 `CheckToolAccess` 反而无法正确阻断（走 `identity_not_found` 分支，效果等同于 Warn 而非 Revoked 拒绝）。

### 修复

`RevokeAgent` 改为 Get + `Status().Patch()` 将 phase 置为 `Revoked`，不存在时幂等忽略：

```go
identity := &v1alpha1.AgentIdentity{}
if err := s.k8s.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: agentName}, identity); err != nil {
    return k8sclient.IgnoreNotFound(err)
}
if identity.Status.Phase == v1alpha1.AgentPhaseRevoked {
    return nil
}
patch := k8sclient.MergeFrom(identity.DeepCopy())
identity.Status.Phase = v1alpha1.AgentPhaseRevoked
if err := s.k8s.Status().Patch(ctx, identity, patch); err != nil {
    return fmt.Errorf("patch AgentIdentity status: %w", err)
}
```

同步更新 `handleIsolationAgentRevoke` 的注释（"deleting its AgentIdentity CRD" → "setting its AgentIdentity phase to Revoked"），并补充单元测试 `TestRevokeAgent_SetsRevokedPhase` 和 `TestRevokeAgent_NotFound_IsNoop`。

**效果：**
- 撤销后 CRD 保留，phase=Revoked ✓
- `CheckToolAccess` 能正确识别 Revoked 状态并拒绝后续调用 ✓
- 不存在的 agent 撤销请求幂等处理，不报错 ✓

---

## E2E BeforeAll 超时分析：sandbox restarts=4 + ICE agent nil

### 失败现象

BeforeAll（等待 WG 握手，超时 180s）失败；sandbox pod 日志显示 `restarts=4`，每次重启均报：

```
enrollment token already used
```

companion pod 日志同时出现：

```
receive offer: agent nil, dropping
```

### 根本原因一：凭证无持久化存储

sandbox 二进制在首次注册成功后，将 JWT + WireGuard 私钥写入 `/etc/lattice/sandbox-credentials.json`，用于崩溃恢复时跳过重新注册（`ResumeSandboxViaNATS`）。

`deploySandboxPod`（`test/e2e/helpers_test.go`）创建的 Pod **没有为 `/etc/lattice/` 挂载任何 volume**：

```go
Spec: corev1.PodSpec{
    Hostname:    name,
    HostAliases: hostAliases,
    Containers: []corev1.Container{{...}},
    // Volumes 字段缺失
},
```

K8s + containerd 在容器重启时重置可写层（overlay fs），凭证文件丢失。重启后 `loadSandboxCredentials()` 失败，回退到 `RegisterSandboxViaNATS`，因 enrollment token 是单次使用（`MarkUsed` 已标记），返回 "enrollment token already used" → 进程退出 → 触发下一次重启 → 崩溃循环（`restarts=4`），BeforeAll 180s 内始终无法建立 WG 握手。

### 根本原因二：ICE `Prepare()` 中的 agent 赋值竞态

initiator 侧（companion，peerId 较大）的 `Prepare()` 中，`i.agent` 赋值**未持锁**：

```go
// ice_dialer.go Prepare()
if i.agent == nil {
    agent, err := i.getAgent(remoteId)
    ...
    i.agent = agent  // ← 无锁写入
}
```

而 `Close()`（ICE Failed 状态触发）在锁内将 `i.agent = nil`，`Handle(OFFER)` 也在锁内读取 `i.agent`：

```go
// Handle(OFFER)
i.mu.Lock()
agent := i.agent  // ← 加锁读取
i.mu.Unlock()
if agent == nil {
    i.log.Debug("receive offer: agent nil, dropping")
    return nil
}
```

竞态窗口：`GatherCandidates()`（ACK handler 中调用）触发 ICE 进入采集流程，若 ICE 立即报告 Failed（如单核容器内 tunAdapter 忙轮询占满 CPU，ICE keepalive goroutine 调度延迟），`OnConnectionStateChange` 回调在 OFFER 到达前调用 `i.Close()`，`i.agent` 被置 nil，导致后续所有 OFFER 被静默丢弃，握手永远不完成。

### 修复

**修复一**（`test/e2e/helpers_test.go`）：在 sandbox Pod spec 中增加 `emptyDir` volume，挂载到 sandbox 容器的 `/etc/lattice`。`emptyDir` 在同一 Pod 内跨容器重启持久，Pod 删除时自动清理，恰好满足测试语义。

```go
Volumes: []corev1.Volume{{
    Name: "lattice-config",
    VolumeSource: corev1.VolumeSource{
        EmptyDir: &corev1.EmptyDirVolumeSource{},
    },
}},
// sandbox 容器：
VolumeMounts: []corev1.VolumeMount{{
    Name:      "lattice-config",
    MountPath: "/etc/lattice",
}},
```

**修复二**（`internal/server/transport/ice_dialer.go`）：`Prepare()` 中对 `i.agent` 的读写均在 `i.mu` 内进行，使用 double-checked init 模式，防止并发 `Close()` 与赋值的数据竞争：

```go
i.mu.Lock()
needInit := i.agent == nil
i.mu.Unlock()
if needInit {
    agent, err := i.getAgent(remoteId)
    ...
    i.mu.Lock()
    if i.agent == nil {
        i.agent = agent
    } else {
        _ = agent.Close() // 被抢先，丢弃多余 agent
    }
    i.mu.Unlock()
}
```

**效果：**
- sandbox 崩溃重启后从 `/etc/lattice/sandbox-credentials.json` 恢复凭证，不再重新注册 ✓
- `Prepare()` 与 `Close()` 对 `i.agent` 的并发访问受 `i.mu` 保护 ✓

---

## ICE 竞态完整修复：Dial() / sendPacket(OFFER) / discover()

### 背景

修复 `Prepare()` 竞态后，CI 日志显示两个新现象：

1. sandbox pod 的 panic（`nil pointer dereference` at `ice_dialer.go:381`，即 `agent.StartAccept`）；
2. companion 仍出现 "receive offer: agent nil, dropping"。

说明 `Prepare()` 之外还存在三处裸读 `i.agent` 的竞态窗口未封。

### 竞态点一：Dial() 无锁读 i.agent

```go
// 修复前
case <-i.offerReady:
    if isInitiator(...) {
        iceConn, err = i.agent.StartDial(...)   // 无锁读 → Close() 并发时 i.agent=nil → panic
    } else {
        iceConn, err = i.agent.StartAccept(...) // 同上
    }
    if err = i.agent.AwaitConnect(dialCtx); ...
```

`Close()` 在 `i.mu` 内将 `i.agent = nil`；`Dial()` 在 `offerReady` 触发后直接读裸字段，若 `Close()` 稍早执行，`i.agent` 已为 nil，立即 panic。

**修复**：

```go
case <-i.offerReady:
    i.mu.Lock()
    agent := i.agent
    i.mu.Unlock()
    if agent == nil {
        return nil, ErrDialerClosed  // 转为干净错误，onFailure → restart()
    }
    if isInitiator(...) {
        iceConn, err = agent.StartDial(...)
    } else {
        iceConn, err = agent.StartAccept(...)
    }
    if err = agent.AwaitConnect(dialCtx); ...
```

### 竞态点二：sendPacket(OFFER) 无锁读 i.agent

```go
// 修复前
case grpc.PacketType_OFFER:
    agent := i.agent  // 裸读
    ufrag, pwd, err := agent.GetLocalUserCredentials()
```

`OnCandidate` 回调在 ICE 候选采集期间被 pion 内部 goroutine 调用，`Close()` 可与之并发，`i.agent` 可能在此处被置 nil。

**修复**：

```go
case grpc.PacketType_OFFER:
    i.mu.Lock()
    agent := i.agent
    i.mu.Unlock()
    if agent == nil {
        return fmt.Errorf("agent is nil, cannot send OFFER")
    }
    ufrag, pwd, err := agent.GetLocalUserCredentials()
```

### 竞态点三：discover() 两次裸读 p.iceDialer

```go
// 修复前
go func() {
    if err := p.iceDialer.Prepare(ctx, p.remoteId); err != nil { ... }
    t, err := p.iceDialer.Dial(ctx)  // restart() 可能在 Prepare 和 Dial 之间替换 p.iceDialer
}()
```

`restart()` 在 `p.mu.Lock()` 内替换 `p.iceDialer`，若发生在 `Prepare()` 和 `Dial()` 之间，两次调用作用于不同的 dialer 实例，行为不确定。

**修复**：在进入 goroutine 前（或 goroutine 最开始）用锁固定快照：

```go
p.mu.RLock()
iceD := p.iceDialer
lrpD := p.lrpDialer
p.mu.RUnlock()

go func() {
    if err := iceD.Prepare(ctx, p.remoteId); err != nil { ... }
    t, err := iceD.Dial(ctx)
    ...
}()
```

### 全部竞态点汇总

| 位置 | 竞态 | 修复 |
|------|------|------|
| `Prepare()` | `i.agent` 裸写（initiator 侧），与 `Close()` 并发 | double-checked init + 持锁写 |
| `Dial()` | `i.agent` 裸读，与 `Close()` 并发 → nil panic | 持锁快照 + nil 防卫 |
| `sendPacket(OFFER)` | `i.agent` 裸读，pion OnCandidate goroutine 与 `Close()` 并发 | 持锁快照 + nil 防卫 |
| `discover()` | `p.iceDialer` 二次裸读，与 `restart()` 并发 | 进入 goroutine 前持锁固定快照 |

**关于结果一致性**：加锁后每条路径行为确定——`Dial()` 要么拿到非 nil agent 完成握手，要么返回 `ErrDialerClosed` 走 `restart()` 重试，不再 panic。但 ICE 握手时序受网络影响，E2E 结果仍有随机性。可用 `go test -race ./internal/server/transport/... -count=5` 验证 data race 完全消除。

---

## Sandbox vs 普通 Agent 架构差异深度分析（2026-05-17 续）

### 问题背景

所有 data race 修复后，普通 agent-to-agent E2E 测试通过，但 sandbox↔普通 agent E2E 仍报 ICE 失败。需要系统梳理 sandbox 与普通 agent 的差异。

### 共同点（ICE 代码路径 100% 一致）

| 组件 | 说明 |
|------|------|
| `NewNode` | sandbox 和普通 agent 调用完全相同的函数 |
| `ProbeFactory` / `iceDialer` | 代码完全共用，SYN/ACK/OFFER/ANSWER 逻辑相同 |
| `FilteringUDPMux` | 同一套 UDP demux 机制 |
| `DefaultBind` | WireGuard bind 层完全相同 |
| NATS 信令订阅 | 同一主题格式 `lattice.signals.peers.{peerID}` |

### 结构差异汇总

| 方面 | 普通 agent | sandbox |
|------|-----------|---------|
| 注册 | `ctrClient.Register`（join token，服务端生成密钥） | `RegisterSandboxViaNATS`（enrollment token，客户端生成密钥） |
| TUN 设备 | 内核 TUN（`infra.CreateTUN`） | `tunAdapter`（封装 gVisor channel endpoint） |
| Provisioner | 内核路由 + iptables/eBPF | `sandboxProvisioner`（ApplyRoute/ApplyIP/SetupNAT 全部 no-op） |
| 心跳 | `go c.StartHeartbeat(gCtx)` | **缺失** |
| LRP relay | `--enable-lrp` 标志可选 | **默认关闭** |
| UAPI socket | 有 | 无 |
| 注册时携带 Port | `config.Conf.WgPort`（51820） | `dto.PeerDto.Port` **未设置（0）** |
| `tunAdapter.Events()` | 内核 TUN 返回真实 events channel | **返回 nil** |

### 识别出的 4 个 ICE 前置条件缺陷

#### 缺陷 1：`tunAdapter.Events()` 返回 nil

wireguard-go 在 `NewDevice` 时启动 `RoutineTUNEventReader` goroutine：

```go
for event := range device.tun.Events() { ... }
```

`range nil` 永久阻塞，goroutine 泄漏。虽然 `device.Up()` 独立工作，但 wireguard-go 要求 TUN 发出 `EventUp` 来触发内部 TUN 读写 goroutine 完整激活，nil channel 导致这一信号永远不发出。

**修复**（`internal/agent/gvisor/wg_device.go`）：

```go
func NewTUNAdapter(ch *channel.Endpoint, inject func(packet []byte) error) tun.Device {
    events := make(chan tun.Event, 1)
    events <- tun.EventUp  // 立即发送 EventUp
    close(events)          // 关闭 channel → RoutineTUNEventReader goroutine 正常退出
    return &tunAdapter{ch: ch, inject: inject, mtu: 1500, events: events}
}

func (t *tunAdapter) Events() <-chan tun.Event { return t.events }
```

#### 缺陷 2：注册时 Port = 0

`RegisterSandboxViaNATS` 发给服务端的 `dto.PeerDto` 不携带 `Port`，服务端记录的 LatticePeer endpoint 端口为 0，可能影响 companion 侧的 peer 元数据。

**修复**（`internal/agent/sandbox_register.go`）：

```go
regPayload, _ := json.Marshal(&dto.PeerDto{
    AppID:     agentName,
    Token:     enrollmentToken,
    PublicKey: pubKey,
    Port:      51820,  // 补上
})
```

同步在 `sandbox_pro.go` 的 `runStart` 中补 `agentconfig.Conf.WgPort = 51820`，保持配置一致。

#### 缺陷 3：没有心跳

普通 agent 每 30s 发一次心跳。Sandbox 无心跳，服务端在心跳超时后将节点标记 offline 并从其他节点的 `ComputedPeers` 移出，导致 companion 的 `peerManager.GetIdentity()` 查不到 sandbox，ICE SYN 被静默丢弃：

```go
// probe_factory.go
remoteIdentity, ok := p.peerManager.GetIdentity(remoteId)
if !ok {
    p.log.Warn("dropping signal packet from unknown peer...")
    return nil
}
```

**修复**（`cmd/lattice/cmd/sandbox/sandbox_pro.go`）：

```go
if err = node.Start(ctx); err != nil {
    return fmt.Errorf("start node: %w", err)
}
go node.StartHeartbeat(ctx)  // 新增
```

#### 缺陷 4：无 LRP 兜底

普通 agent 可配置 LRP relay 作为 ICE 打洞失败的备选通道。Sandbox 的 `agentconfig.Conf.EnableLrp` 默认 false，ICE 一旦失败只能等待 restart，无降级路径。

**修复**（`cmd/lattice/cmd/sandbox/sandbox_pro.go`）：注册完成后检查服务端返回的 `currentPeer.LrpUrl`，非空时自动启用：

```go
if currentPeer.LrpUrl != "" {
    agentconfig.Conf.EnableLrp = true
    agentconfig.Conf.RelayURL = currentPeer.LrpUrl
}
```

### 修复后 ICE 一致性评估

| 维度 | 修复前 | 修复后 |
|------|--------|--------|
| ICE 协商代码 | 100% 共用 | 100% 共用（不变） |
| peer 可见性 | 心跳缺失 → 可能被移出 ComputedPeers | 持续心跳 → 始终可见 ✓ |
| Port 元数据 | 注册时 Port=0 | Port=51820 ✓ |
| ICE 失败兜底 | 无 relay 备选 | LRP fallback（与普通 agent 等价） ✓ |
| TUN 激活信号 | Events()=nil → goroutine 泄漏 | EventUp 发出 → goroutine 正常退出 ✓ |

ICE 协商前置条件已与普通 agent 对齐。剩余差异（gVisor TUN 数据面、no-op routes）是有意的架构设计，不影响 ICE 过程本身。
