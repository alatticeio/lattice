# Transport 层设计文档

## 概述

Wireflow 的 Transport 层负责在两个 Peer 之间自动建立并维护加密通信通道。它抽象了底层连通性的差异，对上层（WireGuard 引擎）透明地提供两种传输路径：

- **ICE**（Interactive Connectivity Establishment）：基于 pion/ice 的 P2P 直连通道，穿越 NAT
- **WRRP**（Wireflow Relay & Routing Protocol）：通过中继服务器转发的隧道通道，作为直连的降级备选

两条路径并行竞争建连，优先使用直连；直连失败时无缝降级到中继；直连建立后自动升级替换中继。

---

## 三层架构

Transport 层由三个职责清晰的层次组成：

```
┌─────────────────────────────────────────────────────────────┐
│                       ProbeFactory                          │
│           管理所有远端 Peer 的 Probe，路由 Signal 包            │
├──────────────────────────┬──────────────────────────────────┤
│         Probe            │  每个远端 Peer 一个 Probe 实例      │
│  负责连接生命周期管理        │  竞速 / 升级 / 断线重连            │
├─────────────┬────────────┴──────────────────────────────────┤
│  ICE Dialer │  WRRP Dialer  │  负责单一协议的握手与建连         │
│   (直连)    │   (中继)      │  持有 Signal 包处理逻辑          　│
├─────────────┴─────────────────────────────────────────────  ┤
│  ICETransport  │  WrrpTransport  │  建连完成后的数据读写抽象    │
└─────────────────────────────────────────────────────────────┘
                            │
                     Signal Service
                   (NATS 信令通道)
```

### Probe 层

**职责**：管理与单个远端 Peer 的完整连接生命周期。

- 同时启动 ICE 和 WRRP 两路 Dialer，竞速取胜者
- 持有当前活跃的 `Transport`，负责升级（WRRP→ICE）
- 监听 Dialer 的 close 事件，触发自动重连（`restart()`）
- `discover()` 全部失败时，通过 `onFailure` 回调调度延迟重试
- 连接成功后调用 `onSuccess` 将端点配置写入 WireGuard（AddPeer + 路由）

### Dialer 层

**职责**：负责单一传输协议的信令握手与连接建立。

- `Prepare()`：初始化本端状态，按角色决定是否主动发起握手
- `Handle()`：处理信令包（SYN / ACK / OFFER / ANSWER），驱动握手状态机
- `Dial()`：阻塞等待握手完成，返回可读写的 `Transport`

Dialer 是**一次性**的——一个 Dialer 实例对应一次建连尝试。连接断开后由 Probe 通过工厂函数创建新实例重试。

### Transport 层

**职责**：对已建立连接的数据平面抽象。

- 提供统一的 `Read` / `Write` 接口，屏蔽 ICE 与 WRRP 的底层差异
- `Priority()` 字段驱动升级逻辑（值越大优先级越高）
- WireGuard 引擎通过 `conn.Endpoint` 接口透明使用

---

## 核心接口

### Transport

```go
type Transport interface {
    Write(data []byte) error
    Read(buff []byte) (int, error)
    RemoteAddr() string
    Type() TransportType   // ICE | WRRP
    Priority() uint8       // 值越大优先级越高，驱动升级决策
    Close() error
}
```

优先级常量：

| 类型 | 常量 | 值 |
|------|------|----|
| ICE P2P 直连 | `PriorityDirect` | 100 |
| WRRP 中继 | `PriorityRelay` | 50 |

### Dialer

```go
type Dialer interface {
    Prepare(ctx context.Context, remoteId PeerIdentity) error
    Handle(ctx context.Context, remoteId PeerIdentity, pkt *SignalPacket) error
    Dial(ctx context.Context) (Transport, error)
    Close() error
    Type() DialerType   // ICE_DIALER | WRRP_DIALER
}
```

注意 `Dial()` 不接收 `remoteId`，因为 Dialer 在创建时已与特定 remoteId 绑定（一 Dialer 实例对应一个 Peer）。`Close()` 用于主动释放 Dialer 持有的所有资源（ICE Agent、goroutine 等），由 Probe 在生命周期结束时调用。

### Probe

```go
type Probe interface {
    Start(ctx context.Context, remoteId PeerIdentity) error
    Handle(ctx context.Context, remoteId PeerIdentity, pkt *SignalPacket) error
    Ping(ctx context.Context) error
}
```

---

## Peer 身份标识

### PeerID

8 字节紧凑标识符，取 WireGuard 公钥前 8 字节：

```
WireGuard PublicKey (32 bytes)
    └─ [0:8] → PeerID (uint64)
               用于 NATS Subject 路由 & 协议帧 sender_id 字段
```

### PeerIdentity

统一逻辑身份（AppID）与加密身份（WireGuard PublicKey）：

| 层 | 使用的标识 |
|----|-----------|
| Management 层 | AppID（字符串，human-readable） |
| NATS 路由 | PeerID（uint64，取公钥前 8 字节） |
| WireGuard 配置 | PublicKey（完整 32 字节） |

### PeerManager

双索引线程安全存储，两个方向均 O(1) 查找：

```
peers: AppID  → Peer
byID:  PeerID → Peer
```

ProbeFactory 在收到 Signal 包时，先用包中的 `sender_id`（PeerID）查询 PeerManager 还原完整 PeerIdentity，再路由到对应 Probe。

---

## ProbeFactory

Transport 层的入口，核心职责：

1. **Probe 生命周期管理**：按 AppID 维护 Probe 实例映射，`Get()` 自动创建不存在的 Probe
2. **Signal 包路由**：`Handle()` 收到信令后，用 `sender_id` 解析出完整 `PeerIdentity`，分发给对应 Probe
3. **Dialer 工厂注入**：`makeIceDialer` 闭包捕获 Probe 引用，使新建的 Dialer 能在 close 时回调 `probe.restart()`

```
ProbeFactory.Handle(senderId PeerID, pkt)
    └─ PeerManager.GetIdentity(senderId) → PeerIdentity
    └─ ProbeFactory.Get(remoteIdentity)  → Probe（不存在则创建）
    └─ Probe.Handle(pkt)
```

---

## Probe 连接建立流程

### 竞速建连（discover）

```
Probe.Start()
    ├─ goroutine A: ICE Dialer.Prepare() → Dial()   ──┐
    │                                                  ├─ 谁先成功谁赢
    └─ goroutine B: WRRP Dialer.Prepare() → Dial()  ──┘
                                │
              for-select 收集结果 / 错误
              ├─ result 到达 → 返回 Transport
              ├─ 全部 error → 返回 lastErr → onFailure
              └─ ctx.Done() → 返回 ctx.Err()
```

### 竞速策略（WRRP 赢后等待）

```
ICE 先成功  → 立即使用，不等 WRRP
WRRP 先成功 → 等 500ms 给 ICE 机会
    ├─ 500ms 内 ICE 也成功 → Close(WRRP)，使用 ICE
    └─ 超时          → 先用 WRRP，ICE 后续建成时升级
```

### Transport 升级

ICE 在 WRRP 之后建成时，通过 `handleUpgradeTransport` 升级：

```go
if newTransport.Priority() > currentTransport.Priority() {
    // 替换为高优先级 Transport
    // 延迟 2s 关闭旧连接，等待缓冲区排空
    // 重新调用 onSuccess 更新 WireGuard endpoint
}
```

### 失败重试与 Probe 生命周期

`discover()` 全部路径均失败时，调用 `onFailure`。Probe 的存活时间与 ConfigMap 中的 Peer 成员资格绑定：

```
discover() 全部失败
    └─ onFailure(err)
           ├─ 记录首次失败时间 firstFailureAt（成功连接后重置）
           ├─ elapsed < 60s → time.AfterFunc(10s, probe.restart)
           │                        └─ 创建新 iceDialer + 重新 Start()
           └─ elapsed ≥ 60s → ProbeFactory.Remove(appId)
                                    └─ probe.Close() + 从 map 中删除
                                    └─ 等待管理服务器推送 PeersAdded 重建 Probe
```

**设计原则**：Probe 不无限重试。对端离线超过 60s 后，本端认为对端已下线，主动关闭 Probe 释放资源。重连的触发权交给管理服务器：服务器检测到节点重新上线后推送 `PeersAdded`，Agent 收到后通过 `AddPeer()` 创建全新 Probe。

**另一条关闭路径**：管理服务器在检测到节点离线后推送 `PeersRemoved` 事件，Agent 的 `RemovePeer()` 会调用 `ProbeFactory.Remove()`，立即关闭 Probe，无需等待 60s 自关闭。两条路径互为兜底。

---

## ICE 握手协议

### 角色分配

握手角色由两端 PeerID 的**字典序比较**静态决定，无需额外协商：

```
PeerID_local > PeerID_remote → 本端是 Initiator（主动发 SYN）
PeerID_local < PeerID_remote → 本端是 Responder（等待 SYN，回 ACK）
```

两端独立计算，得出互补的一致结论。

### 握手时序

```
Initiator (大 PeerID)                    Responder (小 PeerID)
    │                                           │
    │── HANDSHAKE_SYN（立即发，之后每 2s 重试）──▶│
    │◀─ HANDSHAKE_ACK ─────────────────────────│
    │   [Initiator 停止重试，双方开始 GatherCandidates]
    │                                           │
    │── OFFER (ufrag/pwd/TieBreaker/candidate)─▶│
    │◀─ OFFER (ufrag/pwd/TieBreaker/candidate)──│  (双方互发，每个 candidate 一包)
    │                                           │
    │   [offerReady 触发，进入 Dial/Accept]       │
    │                                           │
    │◀═══════════ ICE P2P Connection ═══════════│
```

**SYN 首包立即发送**：`Prepare()` 在启动重试 ticker 之前先发一个 SYN，不等待第一个 2s tick，减少建连延迟。SYN 最多持续发送 60s（context timeout）。

**Dial 超时**：`Dial()` 内置 65s 超时（略大于 SYN 窗口 60s）。对于 Responder 侧，`Prepare()` 直接返回、`Dial()` 阻塞等待对端 SYN——若 65s 内没有收到任何 OFFER，`Dial()` 返回错误，触发 `onFailure` 进入失败计时逻辑。

OFFER 包同时携带：ICE 凭证（ufrag/pwd）、TieBreaker（决定 Dial/Accept 角色）、candidate 地址、本端 Peer 元信息（IP、公钥，用于对端 WireGuard 配置）。

### Dial/Accept 角色

ICE 底层的 Dial/Accept 角色由 **TieBreaker** 决定（与握手 SYN/ACK 角色独立）：

```
local.TieBreaker > remote.TieBreaker → agent.Dial(RUfrag, RPwd)
local.TieBreaker < remote.TieBreaker → agent.Accept(RUfrag, RPwd)
```

TieBreaker 在创建 Agent 时随机生成，概率上两端不同。

---

## iceDialer 生命周期

```
NewIceDialer()
    │
    ▼
Prepare(ctx, remoteId)
    ├─ 创建 ice.Agent（OnConnectionStateChange、OnCandidate 注册）
    ├─ 若是 Initiator：立即发第一个 SYN，然后每 2s 重试（最多 60s）
    └─ 若是 Responder：直接返回，等待对端 SYN

Handle(SYN/ACK/OFFER)
    ├─ SYN  → 发 ACK，创建 Agent，GatherCandidates
    ├─ ACK  → 取消 SYN 重试，GatherCandidates
    └─ OFFER → 添加 remote candidate，触发 offerReady

Dial(ctx)                          ← 内置 65s 超时
    └─ 阻塞等待 offerReady，然后 Dial/Accept

OnConnectionStateChange(Disconnected/Failed)
    └─ Close() → 触发 onClose → probe.restart()
```

### STUN 服务器配置

ICE Agent 使用以下 STUN 服务器收集 srflx（Server-Reflexive）候选，用于 NAT 穿越：

```
优先级顺序：
1. stun.l.google.com:19302   （Google，主用）
2. stun1.l.google.com:19302  （Google，备用）
```

内置 STUN（`stun.wireflow.run`）需在部署时独立配置端口和域名后方可加入列表；未配置时不应填入，否则会因 STUN 超时导致 srflx 候选缺失，跨网络节点无法打洞。

### 断线重连

```
连接断开（ICE keepalive 超时 / 网络中断）
    └─ ice.Agent.OnConnectionStateChange(Failed)
           └─ iceDialer.Close()
                  ├─ closed.Store(true)
                  ├─ agent.Close()
                  ├─ close(closeChan)  — 解除 Dial() 阻塞
                  └─ onClose() → probe.restart()
                                      ├─ p.iceDialer = newIceDialer()
                                      ├─ p.started.Store(false)
                                      └─ p.Start()  — 重新走完整流程

restart() 调用前检查 newIceDialer != nil：
    newIceDialer == nil 说明 Probe.Close() 已被调用（主动关闭），直接返回，不再重建。
```

### Probe.Close() 关闭顺序

```
Probe.Close()
    ├─ mu.Lock()
    ├─ newIceDialer = nil   ← 防止 onClose 回调触发 restart()
    ├─ d = iceDialer; iceDialer = nil
    ├─ mu.Unlock()
    └─ d.Close()            ← iceDialer.Close() 触发 onClose，但 restart() 因 nil 检查提前返回
```

### 快速重启保护（Fast-Restart Race）

**问题**：对端快速重启时（比 ICE keepalive 超时更快），可能出现两种竞态：

1. **Stale agent**：本端 ICE keepalive 尚未超时（agent 仍为 Connected），对端已重启并发来新的 SYN。若复用旧 agent，凭证不匹配导致打洞失败。

2. **Zombie dialer**：本端 `close()` 已执行（`closed=true`，probe 已 restart），但旧 iceDialer 的 SYN 包（网络延迟）仍在路上，打到已关闭的旧实例。

**解决方案**：

```
Handle(SYN) 收到包时：
    ├─ closed == true → 丢弃（新 iceDialer 会处理下次重试）
    ├─ agent != nil   → 对端重启，强制 go close() → probe.restart()
    └─ 正常路径：发 ACK，创建新 Agent
```

`close()` 在 `closeOnce.Do` 里第一步设置 `closed.Store(true)`，保证此后任何迟到的 SYN/ACK/OFFER 都被安全丢弃。

---

## WRRP 握手协议

流程与 ICE 类似，使用相同的信令包类型：

```
Initiator                               Responder
    │── HANDSHAKE_SYN ─────────────────────▶│
    │◀─ HANDSHAKE_ACK ──────────────────────│
    │── OFFER (peer info: IP/PublicKey) ───▶│
    │◀─ ANSWER ─────────────────────────────│
    │◀══════════ WRRP Relay Stream ═════════│
```

OFFER 中携带本端 Peer 元信息，供中继服务器识别和路由流量。

---

## 信令层

Signal 包使用 Protobuf 序列化，通过 NATS 主题点对点路由：

```
主题格式: peer.<PeerID>

SignalPacket:
    type      - HANDSHAKE_SYN / HANDSHAKE_ACK / OFFER / ANSWER / MESSAGE
    dialer    - ICE / WRRP  （区分两条握手路径）
    sender_id - uint64 PeerID
    payload   - Handshake | Offer | Message
```

`dialer` 字段让同一条 NATS 主题上的 ICE 和 WRRP 信令互不干扰。

---

## 连接成功后的配置写入

Transport 建立成功后，Probe 的 `onSuccess` 回调将连接信息写入 WireGuard：

```
onSuccess(transport)
    ├─ provisioner.AddPeer(SetPeer{
    │      PublicKey:  remoteId.PublicKey,
    │      Endpoint:   transport.RemoteAddr(),   // ICE: "ip:port"
    │      AllowedIPs: remotePeer.AllowedIPs,    // WRRP: "wrrp://peerId"
    │  })
    ├─ provisioner.ApplyRoute("add", remoteIP, ifaceName)
    └─ provisioner.SetupNAT(interfaceName)
```

Transport 升级时（WRRP→ICE）重新调用 `onSuccess`，WireGuard Endpoint 从中继地址切换为直连地址，流量路径自动更新。

---

## 设计决策

| 决策 | 原因 |
|------|------|
| ICE + WRRP 并行竞争 | 不等 ICE 超时才降级，最大化连接速度 |
| WRRP 赢后等 500ms | 给 P2P 打洞一个合理时间窗口，避免直接使用低优先级通道 |
| Dialer 一次性，断开即重建 | 状态简单，不需要 Reset 逻辑；复用旧 Agent 凭证会失效 |
| 字典序决定握手角色 | 两端独立计算，无需额外协商，天然防冲突 |
| TieBreaker 决定 Dial/Accept 角色 | 与握手角色解耦，随机生成避免固定依赖 |
| SYN 收到时检查 agent 是否存在 | 防止对端快速重启时复用旧 Agent 导致打洞失败 |
| closed atomic.Bool | 保证迟到的信令包在 close 后被安全丢弃，不污染新 Dialer |
| SYN 立即发第一包 | ticker 首 tick 需等 2s，导致建连延迟；先发一包再走重试逻辑 |
| Dial() 65s 超时 | Responder 侧无主动发包，若对端离线则 Dial 会永久阻塞；超时后触发 onFailure |
| Probe 60s 后自关闭 | 避免对大量离线节点持续空跑 goroutine；重连权交给 ConfigMap 驱动 |
| PeersRemoved 立即关闭 Probe | 管理服务器检测下线后推送事件，Agent.RemovePeer() 调用 Remove() 即时释放 |
| Probe.Close() 先置 newIceDialer=nil | 防止 iceDialer.Close() 触发 onClose 回调后 restart() 重建本应关闭的 Probe |
| STUN 使用 Google 公共服务器 | 内置 STUN 需独立部署配置，未配置时域名无法解析导致 srflx 候选缺失 |
| PeerID 取公钥前 8 字节 | 紧凑（8 字节 vs 32 字节），适合 NATS 主题和协议帧编码 |
| Priority 字段统一升降级逻辑 | 未来新增传输类型只需设置优先级值，升级代码无需修改 |
