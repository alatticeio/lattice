# ICE 握手协议与连接建立

> 生成日期: 2026-05-21（基于 `internal/server/transport/` 代码实现）

## 概述

Lattice 节点间通过**双路径并发竞争**建立 WireGuard 隧道：
- **ICE 直连**（首选）：UDP 打洞，pion/ice v4
- **LRP relay**（回退）：QUIC datagram + TCP HTTP-upgrade，中继转发

NATS 作为信令通道传递握手消息。ICE 和 WireGuard 共享同一个 UDP 端口 `:51820`（`FilteringUDPMux` 解复用 STUN 和 WireGuard 流量）。

---

## 角色确定

```
isInitiator(local, remote) = local.ID().ToUint64() > remote.ID().ToUint64()
```

使用 **uint64 数值比较**（非字符串），避免 `"9" > "14"` 的字典序 bug。

- **initiator**：较大的 ID → 主动发 SYN、驱动 OFFER/ANSWER、设 PersistentKeepalive
- **responder**：较小的 ID → 等待 SYN，回复 ACK

---

## 信令消息（4 种，通过 NATS）

| 消息类型 | 方向 | 载荷 | 触发时机 |
|----------|------|------|---------|
| `HANDSHAKE_SYN` | initiator → responder | local peer_info（Address, AllowedIPs） | Prepare() 立即发送，之后每 2s ticker 重发 |
| `HANDSHAKE_ACK` | responder → initiator | local peer_info | 收到 SYN 后，ICE agent 初始化完成后发送 |
| `OFFER` | 双向 | local ICE candidate + Current peer_info（向后兼容） | OnCandidate 回调触发，每个 local candidate |
| `ANSWER` | 双向 | remote ICE candidate | 与 OFFER 共用 Handle 逻辑 |

---

## 握手时序（ICE 成功路径）

```
Initiator (localId 更大)                      Responder (localId 更小)
══════════════════════════                    ═════════════════════════

Prepare():                                    Prepare():
  getAgent() → 创建 ICE agent                   isInitiator = false → 直接返回
  GatherCandidates()                          （等待 SYN）

  ──────── HANDSHAKE_SYN ────────►
  (含 peer_info: Address, AllowedIPs)

                                          Handle(SYN):
  ◄─────── HANDSHAKE_ACK ────────           1. 提取 peer_info（只执行一次，防重入）
  (含 peer_info)                             2. getAgent() → 创建 ICE agent
                                             3. send ACK

Handle(ACK):
  1. 提取 peer_info（只执行一次，防重入）
  2. GatherCandidates()（若未 gather）
  ──────── OFFER(candidate) ────────►      Handle(OFFER):
                                              AddRemoteCandidate()
  ◄──────── ANSWER(candidate) ───────        ◄── 或直接 OFFER（因 GatherCandidates
                                                在 ACK 后执行，responder 可能先 gather）

Handle(ANSWER/OFFER):
  AddRemoteCandidate()

Dial():                                      Dial():
  AwaitConnect()                              AwaitConnect()
    ↓                                           ↓
  ◄══════ ICE Connected ═══════►             ◄══════ ICE Connected ═══════►

Agent.Close()                                Agent.Close()
Set WireGuard endpoint                       Set WireGuard endpoint
```

### 重传处理

- **SYN 重传**：initiator 每 2s 重发 SYN（ticker），收到 ACK 后取消 ticker
- **SYN + active agent**（responder）：responder 已有活跃 ICE agent 时收到 SYN → 视为 retransmit，重发 ACK，重发 candidates
- **SYN + closed agent**（responder）：responder 的 ICE agent 已关闭时收到 SYN → 认为 remote 重启，触发 `restart()` 重建 probe
- **late ACK/SYN**：重发的 ACK 在 agent 初始化后到达 → `resendCandidates()` 重新发送所有本地 candidates

---

## 并发竞争与 LRP 回退

`Probe.discover()` 并发启动 ICE 和 LRP 两个 dialer：

```
discover():
  ┌─ goroutine: iceD.Prepare() → iceD.Dial()
  └─ goroutine: lrpD.Prepare() → lrpD.Dial()   (EnableLrp=true 时)

  result  ← 首个成功的 transport

  if result == LRP:
    ┌─ 等待 500ms，ICE 也可能成功
    │  if ICE 在 500ms 内成功: 关 LRP，return ICE
    └─ else: lrpWon=true，ICE 后续成功时 upgrade

  if 所有 dialer 失败: return lastErr
```

### LRP → ICE 升级

当 LRP 先成功但 ICE 后续到达时，`handleUpgradeTransport()` 透明切换到 ICE 直连，不重设 WireGuard peer（仅更新 endpoint）。

---

## 连接状态机

```
                ┌─────── onSuccess(ICE) ────────► ICEReady ─┐
                │                                            │
Created → Probing                                            ├→ Failed → Probing (重试)
                │                                            │         → Closed (60s)
                └─────── onSuccess(LRP) ────────► LRPReady ─┘
                                                    │
                                                    └→ ICEReady (upgrade)
```

| 状态 | 含义 | 允许转换 |
|------|------|---------|
| `created` | Probe 已分配 | `probing` |
| `probing` | 双路径竞争 | `ice-ready`, `lrp-ready`, `failed` |
| `ice-ready` | ICE 直连成功 | `failed`, `closed` |
| `lrp-ready` | LRP relay 成功 | `ice-ready`（升级）, `failed`, `closed` |
| `failed` | 所有 dialer 失败 | `probing`（10s 后）, `closed` |
| `closed` | 60s 不可达 | 无 |

### 失败重试策略

| 条件 | 行为 |
|------|------|
| `ErrDialerClosed`（remote 重启触发 close） | 立即 restart，不等 10s |
| 其他错误 | 记录 `firstFailureAt`，10s 后 restart |
| `firstFailureAt` ≥ 60s | 转 `Closed`，不重试 |

### 状态回调

```
Probing → ICEReady/LRPReady:  SetEndpoint + ApplyRoute + SetupNAT
LRPReady → ICEReady (upgrade): SetEndpoint only
→ Failed / → Closed:           RemovePeer
```

---

## FilteringUDPMux 共享端口

ICE STUN 和 WireGuard 加密流量共享 UDP `:51820`。两个 goroutine 对同一 `net.UDPConn` 的竞争：

| 包类型 | ICE agent | WireGuard |
|--------|-----------|-----------|
| STUN（magic `0x2112A442`） | ✅ 正确分派 | ❌ decrypt 失败，丢弃 |
| WireGuard 加密包 | ❌ 无 ufrag 匹配，丢弃 | ✅ 正确处理 |

`FilteringUDPMux` (`internal/agent/infra/mux_filter.go`) 检查字节 4-7 的 STUN magic cookie，将 STUN 分派给 ICE agent，其余给 WireGuard `DefaultBind`。

ICE 连接成功后 agent 关闭，WireGuard 的 `PersistentKeepalive` 维护 NAT 映射。

---

## 关键代码文件

| 文件 | 职责 |
|------|------|
| `internal/server/transport/state_machine.go` | `StateMachine` + `PeerState` 枚举 |
| `internal/server/transport/ice_dialer.go` | ICE dialer：消息处理、agent 生命周期 |
| `internal/server/transport/probe.go` | `Probe`：双路径竞争、discover() |
| `internal/server/transport/probe_factory.go` | `ProbeFactory`：probe 创建/缓存、回调 |
| `internal/server/transport/lrp_dialer.go` | LRP relay dialer（QUIC + TCP） |
| `internal/server/transport/role.go` | `isInitiator()` 角色判定 |
| `internal/agent/infra/mux_filter.go` | `FilteringUDPMux`：STUN/WG 解复用 |
