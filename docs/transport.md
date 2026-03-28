# Transport 层设计文档

## 概述

Wireflow 的 Transport 层负责在两个 Peer 之间建立加密通信通道。它支持两种传输路径：

- **ICE**（Interactive Connectivity Establishment）：基于 pion/ice 的直连 P2P 通道
- **WRRP**（Wireflow Relay & Routing Protocol）：通过中继服务器转发的隧道通道

两者并行竞争，优先使用直连，降级时自动切换到中继。

---

## 架构总览

```
┌─────────────────────────────────────────────────────────────────┐
│                        ProbeFactory                             │
│  管理所有远端 Peer 的 Probe 实例，路由 Signal 包                    │
│                                                                 │
│   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐       │
│   │  Probe(A↔B)  │   │  Probe(A↔C)  │   │  Probe(A↔D)  │ ...   │
│   └──────┬───────┘   └──────────────┘   └──────────────┘       │
│          │                                                      │
│   ┌──────┴────────────────────────┐                             │
│   │        ICE Dialer             │   WRRP Dialer               │
│   │  (pion/ice 直连)               │   (中继隧道)                  │
│   └───────────────────────────────┘                             │
└─────────────────────────────────────────────────────────────────┘
                          │
                    Signal Service
                  (NATS 信令通道)
```

---

## 核心接口

### Transport

```go
type Transport interface {
    Write([]byte) (int, error)
    Read([]byte) (int, error)
    RemoteAddr() net.Addr
    Type() TransportType   // ICE | WRRP
    Priority() int         // 决定升降级
    Close() error
}
```

优先级常量：

| 类型 | 常量 | 值 |
|------|------|----|
| 直连（未来扩展）| `PriorityDirect` | 100 |
| ICE P2P | `PriorityICE` | 80 |
| WRRP 中继 | `PriorityRelay` | 50 |

### Dialer

```go
type Dialer interface {
    Prepare(ctx context.Context, remoteId string) error
    Handle(ctx context.Context, remoteId string, pkt *SignalPacket) error
    Dial(ctx context.Context, remoteId string) (Transport, error)
    Type() DialerType   // ICE_DIALER | WRRP_DIALER
}
```

### Probe

```go
type Probe interface {
    Start(ctx context.Context) error
    Handle(ctx context.Context, pkt *SignalPacket) error
    Ping() error
}
```

负责管理与单个远端 Peer 的连接发现过程。

---

## Peer 身份标识

### PeerID

8 字节紧凑标识符，取 WireGuard 公钥前 8 字节：

```
WireGuard PublicKey (32 bytes)
    └─ [0:8] → PeerID (uint64)
              用于 NATS Subject 路由 & 协议帧 SenderId 字段
```

### PeerIdentity

统一逻辑身份（AppID）与加密身份（WireGuard PublicKey）：

| 层 | 使用的标识 |
|----|-----------|
| Management 层 | AppID（字符串） |
| Transport 层 | PublicKey（32 字节）/ PeerID（uint64） |

### PeerManager

双索引线程安全存储：

```
peers:  AppID    → Peer
byID:   PeerID   → Peer
```

两个方向均 O(1) 查找。

---

## ProbeFactory

`ProbeFactory` 是 Transport 层的入口，核心职责：

1. **Probe 生命周期管理**：按 `remoteId`（AppID）维护 Probe 实例映射
2. **Signal 包路由**：收到信令后按 PeerID 找到对应 Probe 并分发
3. **Dialer 协调**：持有 ICE Dialer 和 WRRP Dialer 的引用，注入给各 Probe

```
ProbeFactory.Handle(pkt)
    └─ 从 SessionManager 查找 pkt.SenderId → AppID
    └─ 找到对应 Probe
    └─ Probe.Handle(pkt)
```

---

## Probe 连接建立流程

Probe 以**竞争模式**同时启动 ICE 和 WRRP 两条路径：

```
Probe.Start()
    ├─ goroutine: ICE Dialer.Dial()   ──┐
    │                                   ├─ 谁先完成谁赢
    └─ goroutine: WRRP Dialer.Dial()  ──┘
```

### 竞争策略

```
WRRP 成功 → 等待 500ms
    ├─ ICE 也成功 → 使用 ICE（优先级更高），关闭 WRRP
    └─ ICE 未完成 → 使用 WRRP，后续支持升级

ICE 成功  → 立即使用，关闭 WRRP（若已建立）
```

### 状态机

```
New → Checking → Connected → Completed
                     │
                     ├─ Failed
                     ├─ Disconnected
                     └─ Closed
```

---

## ICE 握手协议

握手角色由两端 PeerID 的**字典序比较**决定（避免双方同时充当同一角色）：

```
PeerID_local < PeerID_remote → 本端发起（Initiator）
PeerID_local > PeerID_remote → 本端响应（Responder）
```

握手时序（Initiator 视角）：

```
Initiator                          Responder
    │── HANDSHAKE_SYN ────────────────▶│
    │◀─ HANDSHAKE_ACK ─────────────────│
    │── OFFER (ICE credentials) ──────▶│
    │◀─ ANSWER (ICE credentials) ──────│
    │         [ICE candidate exchange] │
    │◀══════ ICE P2P Connection ═══════│
```

`Agent` 对象封装 pion ice.Agent，存储远端凭证（RUfrag / RPwd / RTieBreaker），并通过原子标志确保凭证只初始化一次。

---

## WRRP 握手协议

流程与 ICE 类似，使用相同的信令类型：

```
Initiator                          Responder
    │── HANDSHAKE_SYN ────────────────▶│
    │◀─ HANDSHAKE_ACK ─────────────────│
    │── OFFER (peer info: IP, keys) ──▶│
    │◀─ ANSWER ────────────────────────│
    │◀══════ WRRP Relay Stream ════════│
```

OFFER 中携带 Peer 元信息（IP 地址、公钥），供中继服务识别和路由。

---

## Session Manager

管理 Session ID（uint64）与 WireGuard 公钥（[32]byte）的双向映射：

```go
idToKey: sync.Map  // uint64    → [32]byte
keyToId: sync.Map  // [32]byte  → uint64
```

Session ID 使用 `crypto/rand` 生成，保证唯一性与不可预测性。用于在 Signal 包中标识连接会话，避免直接传输公钥。

---

## 信令层

Signal 包使用 Protobuf 序列化，通过 NATS 主题路由：

```
主题格式: peer.<PeerID>

包结构:
    Type     - 包类型（HANDSHAKE_SYN/ACK, OFFER, ANSWER）
    SenderId - uint64 PeerID
    Dialer   - 区分 ICE 或 WRRP 路径
    Payload  - 类型特定数据
```

Signal Service 接口：

```go
type SignalService interface {
    Send(peerId PeerID, pkt *SignalPacket) error    // 点对点发送
    Request(peerId PeerID, pkt *SignalPacket) (*SignalPacket, error)  // 请求-响应
    Service() error                                  // 启动服务端监听
}
```

---

## 数据流

建立连接后，WireGuard 数据包通过 Transport 传输：

```
WireGuard Packet
    └─ Transport.Write(pkt)
          ├─ ICETransport  → pion ICE DataChannel → 对端
          └─ WrrpTransport → WRRP Stream → 中继 → 对端
```

`WRRPEndpoint` 实现 `conn.Endpoint` 接口，使 WireGuard 引擎可以透明地使用 WRRP 通道。

---

## 设计决策

| 决策 | 原因 |
|------|------|
| ICE + WRRP 并行竞争 | 尽可能快地建立连接，不等待 ICE 超时才降级 |
| WRRP 赢后等 500ms | 给 ICE 一个合理窗口，避免立即使用低优先级通道 |
| 字典序决定角色 | 无需额外协商，两端独立计算得出一致结论 |
| PeerID 取公钥前 8 字节 | 紧凑（8 字节 vs 32 字节），适合 NATS 主题和协议帧 |
| Session ID 独立于公钥 | 避免在信令中暴露公钥，增强隐私 |
| Priority 字段 | 支持未来扩展更多传输类型，升降级逻辑统一 |
