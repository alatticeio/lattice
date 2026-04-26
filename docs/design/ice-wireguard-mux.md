# ICE 打洞 + WireGuard 共享端口设计文档

> pion/ice v4.2.5 API 版本

---

## 1. 背景与问题

### 1.1 共享端口模式

Wireflow 使用单一 UDP 端口同时承载两类流量：

- **ICE 信令流量**：STUN binding request/response、connectivity check，由 `UDPMuxDefault.connWorker` 管理
- **WireGuard 数据流量**：WireGuard 加密数据包，由 `DefaultBind`（`v4conn`）管理

两者共享同一个 `net.UDPConn`，这是设计的核心约束——对端看到的是同一个 IP:Port，ICE 打洞成功后 WireGuard 可以直接复用该路径。

### 1.2 竞争根因

`UDPMuxDefault` 在构造时启动 `connWorker()` goroutine，它持续从传入的 `net.PacketConn` 读取数据：

```go
// udp_mux.go:285 — UDPMuxDefault 的内部读取循环
func (m *UDPMuxDefault) connWorker() {
    for {
        n, addr, err := m.params.UDPConn.ReadFrom(buf)  // 从 socket 读包
        // 1. 已知地址 → 直接 dispatch 到对应 muxedConn
        // 2. STUN 包 → 按 ufrag 查找 muxedConn
        // 3. 无目标 → m.params.Logger.Tracef("Dropping packet...") + continue (丢弃!)
    }
}
```

而 WireGuard 的 `DefaultBind.makeReceiveIPv4` 也从同一个 `b.v4conn`（即同一个 `net.UDPConn`）读取：

```
net.UDPConn (shared socket)
       │
       ├──→ [goroutine A] UDPMuxDefault.connWorker()    — 消费 STUN 包，但随机拿走 WG 包
       └──→ [goroutine B] DefaultBind.makeReceiveIPv4   — 消费 WG 包，但随机拿走 STUN 包
```

每个 UDP 数据包只能被其中一个 goroutine 读走，产生随机竞争：

| 包类型 | 被 connWorker 拿走 | 被 makeReceiveIPv4 拿走 |
|--------|---------------------|------------------------|
| STUN | 正确分派给 ICE ✓ | FilterMessage 返回 false，WireGuard 解密失败，丢弃 ✗ |
| WireGuard 加密包 | 无匹配 ufrag，connWorker 丢弃 ✗ | WireGuard 正确处理 ✓ |

### 1.3 ICE Agent 生命周期过长

`agent.Dial()`/`Accept()` 成功后，Agent 仍持续运行：

- `connWorker` 继续从 mux 读取，占用穿透路径资源
- Agent 持续向对端发送 STUN keepalive，与 WireGuard 流量混杂
- WireGuard 的 `PersistentKeepalive` 足以维持 NAT 映射，STUN keepalive 是冗余的

### 1.4 现有 AgentWrapper 的冗余逻辑

当前代码使用 `GetTieBreaker()` 与远端 `RTieBreaker` 比较来决定 Dial vs Accept：

```go
// ice_dialer.go (旧实现)
if i.agent.GetTieBreaker() > i.agent.RTieBreaker {
    conn, err = i.agent.Dial(ctx, ufrag, pwd)
} else {
    conn, err = i.agent.Accept(ctx, ufrag, pwd)
}
```

pion/ice v4 中 `tieBreaker` 字段已设为私有，`GetTieBreaker()` 不再存在。
v4 API 明确将角色选择外置：`StartDial`（controlling）/ `StartAccept`（controlled），角色由信令层决定，与 `isInitiator()` 天然对应。

---

## 2. 设计目标

1. **消除竞争**：UDP socket 只有一个读取者，每个数据包只被处理一次
2. **PassThrough 策略**：mux 的 `connWorker` 无法分派的包（无 ICE 目标）→ 转发给 WireGuard，而非丢弃
3. **Wrapper 模式**：不修改 `pion/ice` 原库代码，在上层封装
4. **v4 API 对齐**：使用 `NewAgentWithOptions`、`StartDial`/`StartAccept`、`AwaitConnect`
5. **Agent 生命周期优化**：连接成功后关闭 Agent，WireGuard 独占穿透路径

---

## 3. 整体架构

```
                  ┌─────────────────────────────────────────────────┐
                  │           FilteringUDPMux (Wrapper)             │
                  │          internal/infra/mux_filter.go           │
                  │                                                  │
v4conn ──────────→│  readLoop() — 唯一读取者                        │
(UDP4 共享 socket)│       │                                          │
                  │       │  stun.IsMessage(buf)?                   │
                  │       │                                          │
                  │       ├─ YES → chanConn.inject(buf, addr)       │──→ UDPMuxDefault.connWorker
                  │       │          (chanConn 是给 mux 的假 conn)   │         │
                  │       │                                          │    ICE muxedConn (per ufrag)
                  │       └─ NO  → passThroughCh <- packet          │
                  │                                                  │
                  └──────────────────┬───────────────────────────────┘
                                     │ passThroughCh
                                     ▼
                          DefaultBind.makeReceiveIPv4
                          (从 channel 读，不再直接读 socket)
                                     │
                                     ▼
                               WireGuard Device
                                     ▲
                                     │
                          DefaultBind.makeReceiveIPv6
                          (直接读 v6conn，无 mux，无 channel)
                                     │
v6conn ──────────────────────────────┘
(UDP6 专属 socket，仅 WireGuard 使用)
```

**Agent 关闭后**：

```
v4conn ──→ FilteringUDPMux.readLoop()
                  │
                  ├─ STUN 残留包 → chanConn → mux connWorker（无 ufrag，丢弃）
                  └─ WireGuard 包 → passThroughCh → WireGuard ✓

v6conn ──→ makeReceiveIPv6（始终独占，无变化）──→ WireGuard ✓
```

---

## 4. 组件详细设计

### 4.1 `ChanPacketConn`（新增 `internal/infra/chan_conn.go`）

**作用**：给 `UDPMuxDefault`（及 `UniversalUDPMuxDefault`）传入一个 channel 驱动的假 `net.PacketConn`，让 mux 的 `connWorker` 只消费我们主动注入的数据，不直接读真实 socket，从根本上消除竞争。

```go
// ChanPacketConn 是 channel 驱动的 net.PacketConn。
// 提供给 UDPMuxDefault 使用：mux 的 connWorker 从此读包，
// 我们在 FilteringUDPMux.readLoop 中通过 inject() 主动喂数据。
type ChanPacketConn struct {
    recvCh   chan injectedMsg
    realConn net.PacketConn  // 代理 WriteTo，ICE 发 STUN 响应时使用
    local    net.Addr
    done     chan struct{}
    closeOnce sync.Once
}

type injectedMsg struct {
    data []byte
    addr net.Addr
}

// inject 由 FilteringUDPMux.readLoop 在识别出 STUN 包后调用。
// 将包放入 recvCh，mux 的 connWorker 从中读取。
func (c *ChanPacketConn) inject(data []byte, addr net.Addr) {
    buf := make([]byte, len(data))
    copy(buf, data)
    select {
    case c.recvCh <- injectedMsg{data: buf, addr: addr}:
    case <-c.done:
    }
}

// ReadFrom 由 mux 内部 connWorker 调用（阻塞等待注入数据）。
func (c *ChanPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
    select {
    case msg := <-c.recvCh:
        n = copy(p, msg.data)
        return n, msg.addr, nil
    case <-c.done:
        return 0, nil, net.ErrClosed
    }
}

// WriteTo 代理到真实 socket（mux 发送 STUN 响应/请求时使用）。
func (c *ChanPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
    return c.realConn.WriteTo(p, addr)
}

func (c *ChanPacketConn) LocalAddr() net.Addr        { return c.local }
func (c *ChanPacketConn) SetDeadline(time.Time) error      { return nil }
func (c *ChanPacketConn) SetReadDeadline(time.Time) error  { return nil }
func (c *ChanPacketConn) SetWriteDeadline(time.Time) error { return nil }
func (c *ChanPacketConn) Close() error {
    c.closeOnce.Do(func() { close(c.done) })
    return nil
}
```

> **为何需要代理 `WriteTo`**：`UDPMuxDefault` 发送 STUN binding response 时调用 `m.params.UDPConn.WriteTo()`（`udp_mux.go:253`），必须走真实 socket，否则响应无法到达对端。

### 4.2 `FilteringUDPMux`（新增 `internal/infra/mux_filter.go`）

核心 Wrapper，承担 **唯一读取者** 角色，实现 PassThrough 分派。

```go
// PassThroughPacket 是从 FilteringUDPMux 转发给 WireGuard 的非 STUN 数据包。
type PassThroughPacket struct {
    Data []byte
    Addr *net.UDPAddr
}

// FilteringUDPMux 是 UDPMuxDefault（及 UniversalUDPMuxDefault）的 Wrapper。
//
// 职责：
//   - 成为 UDP socket 的唯一读取者（消除 connWorker 与 makeReceiveIPv4 的竞争）
//   - STUN 包 → inject 到 chanConn → mux connWorker 处理
//   - 非 STUN 包 → passThroughCh → WireGuard DefaultBind 处理
type FilteringUDPMux struct {
    // inner 是给 ice.Agent 使用的 UniversalUDPMuxDefault，
    // 它持有 chanConn（不持有真实 socket）。
    inner    *ice.UniversalUDPMuxDefault
    chanConn *ChanPacketConn

    realConn      net.PacketConn  // 唯一读取者持有的真实 socket
    passThroughCh chan<- PassThroughPacket

    stopCh chan struct{}
    wg     sync.WaitGroup
}

// NewFilteringUDPMux 构造 Wrapper。
// realConn 是真实的 UDP socket（唯一读取者）。
func NewFilteringUDPMux(realConn net.PacketConn, logger logging.LeveledLogger) *FilteringUDPMux {
    chanConn := &ChanPacketConn{
        recvCh:   make(chan injectedMsg, 256),
        realConn: realConn,
        local:    realConn.LocalAddr(),
        done:     make(chan struct{}),
    }

    // 给 mux 传入 chanConn，而非真实 socket。
    // mux 的 connWorker 只从 chanConn 读取（即只消费我们 inject 进来的 STUN 包）。
    inner := ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
        Logger:  logger,
        UDPConn: chanConn,
    })

    return &FilteringUDPMux{
        inner:    inner,
        chanConn: chanConn,
        realConn: realConn,
        stopCh:   make(chan struct{}),
    }
}

// SetPassThrough 注册 WireGuard 的接收 channel，必须在 Start() 之前调用。
func (f *FilteringUDPMux) SetPassThrough(ch chan<- PassThroughPacket) {
    f.passThroughCh = ch
}

// UDPMux 暴露给 ice.WithUDPMux() 使用（host candidate）。
func (f *FilteringUDPMux) UDPMux() ice.UDPMux {
    return f.inner.UDPMuxDefault
}

// UDPMuxSrflx 暴露给 ice.WithUDPMuxSrflx() 使用（server-reflexive candidate）。
func (f *FilteringUDPMux) UDPMuxSrflx() ice.UniversalUDPMux {
    return f.inner
}

// Start 启动唯一读取 goroutine，必须在 SetPassThrough 之后调用。
func (f *FilteringUDPMux) Start() {
    f.wg.Add(1)
    go f.readLoop()
}

// readLoop 是整个系统中唯一从真实 socket 读取数据的 goroutine。
func (f *FilteringUDPMux) readLoop() {
    defer f.wg.Done()
    buf := make([]byte, 1500)

    for {
        n, addr, err := f.realConn.ReadFrom(buf)
        if err != nil {
            select {
            case <-f.stopCh:
                return
            default:
                continue
            }
        }

        udpAddr, _ := addr.(*net.UDPAddr)
        pkt := buf[:n]

        if stun.IsMessage(pkt) {
            // STUN 包 → 注入 chanConn → mux.connWorker 按 ufrag 分派
            f.chanConn.inject(pkt, addr)
        } else {
            // 非 STUN 包（WireGuard 加密包）→ PassThrough → WireGuard
            if f.passThroughCh != nil {
                data := make([]byte, n)
                copy(data, pkt)
                select {
                case f.passThroughCh <- PassThroughPacket{Data: data, Addr: udpAddr}:
                default:
                    // channel 满时丢弃，避免阻塞唯一读取 goroutine
                }
            }
        }
    }
}

func (f *FilteringUDPMux) Close() error {
    close(f.stopCh)
    f.chanConn.Close() //nolint:errcheck  — 唤醒 mux connWorker 退出
    f.wg.Wait()
    return f.inner.Close()
}
```

> **STUN 识别性能**：`stun.IsMessage(buf)` 仅检查 Magic Cookie（4字节比较），O(1)、零分配，适合高频热路径。

### 4.3 `DefaultBind` 修改（`internal/infra/conn.go`）

`makeReceiveIPv4`/`makeReceiveIPv6` 不再直接读取 `v4conn`，改为从 `passThroughCh` 消费：

```go
// 变更前：直接读 socket + FilterMessage 分类
msg.N, msg.NN, _, msg.Addr, err = udpConn.ReadMsgUDP(msg.Buffers[0], msg.OOB)
ok, err := b.universalUdpMux.FilterMessage(msg.Buffers[0], msg.N, msg.Addr.(*net.UDPAddr))
if ok { continue }

// 变更后：从 passThroughCh 消费，已保证是 WireGuard 包
func (b *DefaultBind) makeReceiveIPv4(passThroughCh <-chan infra.PassThroughPacket) conn.ReceiveFunc {
    return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
        pkt, ok := <-passThroughCh
        if !ok {
            return 0, net.ErrClosed
        }
        copy(bufs[0], pkt.Data)
        sizes[0] = len(pkt.Data)
        eps[0] = &WRRPEndpoint{
            Addr:          pkt.Addr.AddrPort(),
            TransportType: ICE,
        }
        return 1, nil
    }
}
```

`BindConfig` 变更：

```go
type BindConfig struct {
    Logger        *log.Logger
    FilteringMux  *infra.FilteringUDPMux  // 替换 UniversalUDPMux + V4Conn/V6Conn
    PassThrough   <-chan infra.PassThroughPacket
    WrrpClient    Wrrp
    KeyManager    KeyManager
}
```

移除 `FilterMessage` 调用，移除 `universalUdpMux` 字段（DefaultBind 不再持有 mux 引用）。

### 4.4 ICE Agent 构造——v4 `NewAgentWithOptions` API

将 `iceDialer.getAgent()` 中的 `ice.NewAgent(&ice.AgentConfig{...})` 替换为 `ice.NewAgentWithOptions(opts...)`:

```go
// 变更前（已废弃的 NewAgent + AgentConfig 结构体）：
iceAgent, err := ice.NewAgent(&ice.AgentConfig{
    InterfaceFilter: func(name string) bool { ... },
    UDPMux:          i.universalUdpMuxDefault.UDPMuxDefault,
    UDPMuxSrflx:     i.universalUdpMuxDefault,
    NetworkTypes:    []ice.NetworkType{ice.NetworkTypeUDP4},
    Urls:            []*stun.URI{{...}},
    Tiebreaker:      uint64(ice.NewTieBreaker()),
    LoggerFactory:   f,
    CandidateTypes:  []ice.CandidateType{...},
    DisconnectedTimeout: &disconnectedTimeout,
    FailedTimeout:       &failedTimeout,
})

// 变更后（v4 NewAgentWithOptions + functional options）：
iceAgent, err := ice.NewAgentWithOptions(
    ice.WithInterfaceFilter(func(name string) bool {
        name = strings.ToLower(name)
        return !strings.Contains(name, "docker") &&
               !strings.Contains(name, "veth") &&
               !strings.Contains(name, "br-") &&
               !strings.HasPrefix(name, "wf")
    }),
    ice.WithUDPMux(i.filteringMux.UDPMux()),
    ice.WithUDPMuxSrflx(i.filteringMux.UDPMuxSrflx()),
    ice.WithNetworkTypes([]ice.NetworkType{ice.NetworkTypeUDP4}),
    ice.WithUrls([]*stun.URI{
        {Scheme: stun.SchemeTypeSTUN, Host: "stun.wireflow.run", Port: 3478},
    }),
    ice.WithCandidateTypes([]ice.CandidateType{
        ice.CandidateTypeHost,
        ice.CandidateTypeServerReflexive,
    }),
    ice.WithDisconnectedTimeout(10*time.Second),
    ice.WithFailedTimeout(15*time.Second),
    ice.WithLoggerFactory(loggerFactory),
)
```

`AgentWrapper` 移除 `RTieBreaker`（v4 不再需要手动比较）：

```go
// 变更前：
type AgentWrapper struct {
    sender              func(ctx context.Context, peerId string, data []byte) error
    *ice.Agent
    IsCredentialsInited atomic.Bool
    RUfrag              string
    RPwd                string
    RTieBreaker         uint64  // ← 删除，v4 API 不再使用
}

// 变更后：
type AgentWrapper struct {
    *ice.Agent
    IsCredentialsInited atomic.Bool
    RUfrag              string
    RPwd                string
}
```

### 4.5 连接建立——v4 `StartDial`/`StartAccept` + `AwaitConnect`

v4 将阻塞式 `Dial`/`Accept` 拆解为非阻塞的 `StartDial`/`StartAccept` 加阻塞的 `AwaitConnect`，更适合异步架构。

**角色决定**：直接复用 `isInitiator(local, remote)`，不再依赖私有的 tiebreaker 比较：
- 本地是 initiator（发 SYN）→ `StartDial`（controlling agent）
- 本地是 responder（收 SYN）→ `StartAccept`（controlled agent）

```go
// 变更前：手动比较 tiebreaker，阻塞式 Dial/Accept
if i.agent.GetTieBreaker() > i.agent.RTieBreaker {
    conn, err = i.agent.Dial(ctx, i.agent.RUfrag, i.agent.RPwd)
} else {
    conn, err = i.agent.Accept(ctx, i.agent.RUfrag, i.agent.RPwd)
}

// 变更后：角色由 isInitiator 决定，使用 StartDial/StartAccept + AwaitConnect
func (i *iceDialer) Dial(ctx context.Context) (infra.Transport, error) {
    select {
    case <-dialCtx.Done():
        return nil, fmt.Errorf("iceDialer: timed out: %w", dialCtx.Err())
    case <-i.closeChan:
        return nil, ErrDialerClosed
    case <-i.offerReady:
    }

    var iceConn *ice.Conn
    var err error

    if isInitiator(i.localId, i.remoteId) {
        // controlling agent：主动发起 STUN binding request
        iceConn, err = i.agent.StartDial(i.agent.RUfrag, i.agent.RPwd)
    } else {
        // controlled agent：等待对端发起，响应 STUN binding request
        iceConn, err = i.agent.StartAccept(i.agent.RUfrag, i.agent.RPwd)
    }
    if err != nil {
        return nil, err
    }

    // AwaitConnect 阻塞，直到至少一对 candidate 连通
    if err = i.agent.AwaitConnect(ctx); err != nil {
        return nil, err
    }

    // 捕获穿透地址（selected candidate pair 的远端地址）
    remoteAddr := ""
    if ra := iceConn.RemoteAddr(); ra != nil {
        remoteAddr = ra.String()
    }
    i.log.Info("ICE connected, handing off to WireGuard", "remoteAddr", remoteAddr)

    // 关闭 Agent：移除 connWorker，让 WireGuard 独占穿透路径。
    // 延迟 500ms 确保最后几轮 STUN connectivity-check 完成，
    // 对端 ICE 不会因突然收不到响应而进入 Failed。
    go func() {
        time.Sleep(500 * time.Millisecond)
        // iceConn.Close() 等价于 agent.Close()（参见 transport.go:229）
        if err := iceConn.Close(); err != nil {
            i.log.Warn("close ICE conn", "err", err)
        }
        i.log.Debug("ICE agent closed, WireGuard owns the path")
    }()

    return &ICETransport{RemoteEndpoint: remoteAddr}, nil
}
```

### 4.6 IPv6 处理

#### 4.6.1 当前设计：v6conn 由 WireGuard 独占

ICE 当前仅启用 UDP4（`ice.WithNetworkTypes([]ice.NetworkType{ice.NetworkTypeUDP4})`），IPv6 socket 不参与 ICE 协商，因此 **v6conn 不存在竞争问题**，设计更简单：

```
v6conn (net.UDPConn, UDP6)
    │
    └──→ makeReceiveIPv6 ──→ WireGuard Device
         （直接读 socket，无 mux，无 channel）
```

`makeReceiveIPv6` 在 Linux 上使用 `ipv6.PacketConn.ReadBatch` 批量收包，其他平台使用 `ReadMsgUDP`，均直接读取 v6conn，无需任何过滤逻辑：

```go
// conn.go — makeReceiveIPv6（简化）
func (b *DefaultBind) makeReceiveIPv6(pc *ipv6.PacketConn, udpConn *net.UDPConn) conn.ReceiveFunc {
    return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
        // 直接读 v6conn，所有包都是 WireGuard 流量，无需 STUN 过滤
        numMsgs, err = pc.ReadBatch(*msgs, 0)  // Linux
        // 或 udpConn.ReadMsgUDP(...)           // 其他平台
        // ...
        eps[n] = &WRRPEndpoint{Addr: addrPort, TransportType: ICE}
    }
}
```

#### 4.6.2 v4 与 v6 接收路径对比

| 维度 | IPv4 (`v4conn`) | IPv6 (`v6conn`) |
|------|-----------------|-----------------|
| ICE 是否使用 | 是（UDP4 host/srflx candidate） | 否（仅 UDP4） |
| 竞争风险 | 有（connWorker vs makeReceiveIPv4） | 无 |
| 读取方式 | 从 `passThroughCh` channel 消费 | 直接读 socket |
| 批量读取（Linux） | 不适用（channel 是串行的） | `ipv6.PacketConn.ReadBatch` |
| STUN 过滤 | 由 `FilteringUDPMux.readLoop` 在注入前完成 | 不需要（无 STUN 流量） |
| 吞吐瓶颈 | channel 容量 512，单包转发 | 批量读，吞吐更高 |

#### 4.6.3 未来扩展：ICE over IPv6

若要支持 ICE 双栈（`ice.NetworkTypeUDP4` + `ice.NetworkTypeUDP6`），v6conn 会同时承载 STUN 流量，竞争问题将在 IPv6 路径上重现，需要以下改动：

**方案 A：为 v6conn 单独创建 FilteringUDPMux**

```go
// 双栈 mux 方案
filterMux4 := infra.NewFilteringMux(v4conn, showLog)
filterMux6 := infra.NewFilteringMux(v6conn, showLog)

passThroughCh4 := make(chan infra.PassThroughPacket, 512)
passThroughCh6 := make(chan infra.PassThroughPacket, 512)
filterMux4.SetPassThrough(passThroughCh4)
filterMux6.SetPassThrough(passThroughCh6)
filterMux4.Start()
filterMux6.Start()

// ICE Agent 同时传入两个 mux
ice.NewAgentWithOptions(
    ice.WithUDPMux(filterMux4.UDPMux()),           // host IPv4
    ice.WithUDPMuxSrflx(filterMux4.UDPMuxSrflx()), // srflx IPv4
    // IPv6 目前 pion/ice 通过 WithUDPMux 同时处理 v4/v6，
    // 或使用独立 WithUDPMuxSrflx 传入 v6 的 UniversalUDPMux
    ice.WithNetworkTypes([]ice.NetworkType{
        ice.NetworkTypeUDP4,
        ice.NetworkTypeUDP6,
    }),
)

// DefaultBind 同时从两个 channel 读取
bind := infra.NewBind(&infra.BindConfig{
    PassThrough4: passThroughCh4,
    PassThrough6: passThroughCh6,
    // ...
})
```

`makeReceiveIPv6` 改为从 `passThroughCh6` 消费，逻辑与 `makeReceiveIPv4` 对称：

```go
func (b *DefaultBind) makeReceiveIPv6() conn.ReceiveFunc {
    return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
        pkt, ok := <-b.passThroughCh6
        if !ok {
            return 0, net.ErrClosed
        }
        sizes[0] = copy(bufs[0], pkt.Data)
        eps[0] = &WRRPEndpoint{Addr: pkt.Addr.AddrPort(), TransportType: ICE}
        return 1, nil
    }
}
```

**方案 B：FilteringUDPMux 扩展为双栈**

将 `FilteringUDPMux` 扩展为同时持有 v4 和 v6 两个 socket，各自一个 `readLoop` goroutine，共用一套 `chanConn`/`inner mux` 逻辑。代码更集中但接口更复杂。

**当前选择**：保持方案 A 的对称结构，仅在需要启用 ICE IPv6 时实施。

### 4.7 `OnSelectedCandidatePairChange` 的使用（可选增强）

v4 提供 `agent.OnSelectedCandidatePairChange(func(local, remote ice.Candidate))` 回调，可在候选对切换时（如从 relay → direct）动态更新 WireGuard endpoint，实现连接升级：

```go
// 注册候选对变化回调（在 getAgent 中）
if err = agent.OnSelectedCandidatePairChange(func(local, remote ice.Candidate) {
    i.log.Info("ICE pair changed",
        "local", local.Address(), "remote", remote.Address())
    // 可触发 WireGuard endpoint 更新（用于 ICE renomination 场景）
}); err != nil {
    return nil, err
}
```

当前方案（Agent 连通后关闭）不需要此回调，但为 ICE renomination 升级路径预留了接口。

### 4.7 `ICETransport` 结构

```go
// ICETransport 持有 ICE 打洞成功后的对端地址，供 onSuccess 回调提取并配置到 WireGuard Peer。
type ICETransport struct {
    RemoteEndpoint string  // 穿透后的对端 IP:Port（如 "1.2.3.4:51820"）
}

func (i *ICETransport) Type() infra.TransportType   { return infra.ICE }
func (i *ICETransport) Priority() uint8              { return infra.PriorityDirect }
func (i *ICETransport) RemoteAddr() string           { return i.RemoteEndpoint }
func (i *ICETransport) Close() error                 { return nil }
func (i *ICETransport) Write(_ []byte) error         { return nil }
func (i *ICETransport) Read(_ []byte) (int, error)   { return 0, nil }
```

---

## 5. 初始化流程

```go
// node/client 启动

// 1. 创建共享 UDP socket
udpConn, port, _ := infra.ListenUDP("udp4", 0)

// 2. 创建 FilteringUDPMux（Wrapper，唯一读取者）
filterMux := infra.NewFilteringUDPMux(udpConn, logger)

// 3. 创建 PassThrough channel（容量根据预期并发连接数调整）
passThroughCh := make(chan infra.PassThroughPacket, 512)
filterMux.SetPassThrough(passThroughCh)

// 4. 启动唯一读取 goroutine（必须在创建任何 Agent 之前）
filterMux.Start()

// 5. 创建 WireGuard Bind，从 passThroughCh 接收包
bind := infra.NewBind(&infra.BindConfig{
    Logger:       logger,
    FilteringMux: filterMux,
    PassThrough:  passThroughCh,
    WrrpClient:   wrrpClient,
    KeyManager:   keyManager,
})

// 6. ICE Dialer 使用 filterMux 提供的 mux 接口
// （agent 构造时通过 WithUDPMux / WithUDPMuxSrflx 传入）
iceDialer := transport.NewIceDialer(&transport.ICEDialerConfig{
    FilteringMux: filterMux,
    // ...
})
```

---

## 6. 数据流时序

### 6.1 ICE 协商阶段


```
对端 STUN 包
    │
    ▼
realConn.ReadFrom()                        [FilteringUDPMux.readLoop]
    │ stun.IsMessage() = true
    ▼
chanConn.inject(buf, addr)
    │
    ▼
UDPMuxDefault.connWorker 读取 chanConn.ReadFrom()
    │ 按 ufrag 查找 muxedConn → dispatch
    ▼
ICE muxedConn.writePacket() → Agent 内部处理 STUN
    │ 发 STUN binding response
    ▼
chanConn.WriteTo() → realConn.WriteTo() → 对端
```

### 6.2 WireGuard 流量（ICE 阶段同期或 Agent 关闭后）

```
对端 WireGuard 加密包
    │
    ▼
realConn.ReadFrom()                        [FilteringUDPMux.readLoop]
    │ stun.IsMessage() = false
    ▼
passThroughCh <- PassThroughPacket{Data, Addr}
    │
    ▼
DefaultBind.makeReceiveIPv4 的 ReceiveFunc
    │ 返回给 WireGuard Device
    ▼
WireGuard 解密 → TUN 接口
```

### 6.3 IPv6 WireGuard 流量

IPv6 路径始终独占 v6conn，没有 mux 介入，流程最简：

```
对端 WireGuard 加密包（IPv6）
    │
    ▼
v6conn.ReadMsgUDP() / ReadBatch()          [makeReceiveIPv6，直接读 socket]
    │ 所有包都是 WireGuard 流量，无需 STUN 判断
    ▼
WireGuard Device 解密 → TUN 接口
```

无 channel、无 mux、无 STUN 过滤，批量读取（Linux）吞吐最高。

### 6.4 Agent 关闭后的残留 STUN 包（对端 Agent 尚未关闭）

```
对端残留 STUN 包
    │
    ▼
chanConn.inject()
    │
    ▼
UDPMuxDefault.connWorker
    │ addressMap 无记录，非 STUN 检查（已是 STUN 包），
    │ 按 ufrag 查找 → 无注册（Agent 已 Close，ufrag 已移除）
    ▼
connWorker: "Dropping packet..." + continue (静默丢弃)
```

残留 STUN 包不会进入 passThroughCh，WireGuard 不受影响。

---

## 7. pion/ice v4 API 变更对照

| 场景 | v3 / 旧写法 | v4 新写法 |
|------|-------------|-----------|
| 构造 Agent | `ice.NewAgent(&ice.AgentConfig{...})` | `ice.NewAgentWithOptions(opts...)` |
| 接口过滤 | `AgentConfig.InterfaceFilter` | `ice.WithInterfaceFilter(func)` |
| UDP Mux（host） | `AgentConfig.UDPMux` | `ice.WithUDPMux(mux)` |
| UDP Mux（srflx） | `AgentConfig.UDPMuxSrflx` | `ice.WithUDPMuxSrflx(mux)` |
| 网络类型 | `AgentConfig.NetworkTypes` | `ice.WithNetworkTypes(types)` |
| STUN 服务器 | `AgentConfig.Urls` | `ice.WithUrls(urls)` |
| 候选类型 | `AgentConfig.CandidateTypes` | `ice.WithCandidateTypes(types)` |
| 超时设置 | `AgentConfig.DisconnectedTimeout` (指针) | `ice.WithDisconnectedTimeout(dur)` |
| 日志工厂 | `AgentConfig.LoggerFactory` | `ice.WithLoggerFactory(factory)` |
| 连接（控制端）| `agent.Dial(ctx, ufrag, pwd)` 阻塞 | `agent.StartDial(ufrag, pwd)` + `agent.AwaitConnect(ctx)` |
| 连接（受控端）| `agent.Accept(ctx, ufrag, pwd)` 阻塞 | `agent.StartAccept(ufrag, pwd)` + `agent.AwaitConnect(ctx)` |
| 角色判断 | `agent.GetTieBreaker()` 比较（私有，v4 不可用）| `isInitiator(local, remote)` 决定 StartDial/StartAccept |
| 关闭连接 | `agent.Close()` | `iceConn.Close()`（等价，conn.Close 调用 agent.Close） |

---

## 8. 关键设计决策

### 8.1 为何在 `readLoop` 分叉而非在 mux 内部加 PassThrough？

`UDPMuxDefault.connWorker` 在无目标时只做 `continue`（丢弃），没有"无目标时回调"的扩展点（参见 `udp_mux.go:352-356`）。修改此处需改动 pion/ice 原库。在 `readLoop` 分叉完全在 Wrapper 层实现，零侵入。

### 8.2 为何用 `isInitiator` 决定 `StartDial`/`StartAccept`？

v4 将 `tieBreaker` 设为私有（`agent.go:59`），`GetTieBreaker()` 不再存在。ICE RFC 8445 中 tiebreaker 用于**冲突解决**，而 wireflow 的 `isInitiator` 基于 PeerID 大小确保两端始终对角色有一致判断，天然满足 RFC 要求（总有一侧更大），且避免了信令层额外传递 tiebreaker。

### 8.3 `StartDial`/`StartAccept` vs `Dial`/`Accept`

pion/ice v4 中 `Dial`/`Accept` 是 `StartDial`/`StartAccept` + `AwaitConnect` 的组合（`transport.go:40-75`）。使用非阻塞的 `StartDial` + 显式 `AwaitConnect(ctx)` 的优点：

- `StartDial` 立即返回 `*Conn`，我们可以在等待的同时继续接收 candidate
- 可对 `AwaitConnect` 单独设置更细粒度的超时
- 更清晰地体现 "启动连接过程" 与 "等待连通" 两个阶段的分离

### 8.4 延迟 500ms 关闭 Agent

ICE RFC 8445 §8.1.2 要求 connectivity check 在选定对后仍持续若干轮。延迟 500ms：
- 让双方完成最后的 check 轮次，对端 ICE 优雅退出
- WireGuard 发出第一个 keepalive，NAT 映射稳定
- 之后 `iceConn.Close()` → `agent.Close()` → `removeUfragFromMux()` → chanConn 通知 done

### 8.5 IPv6 为何不需要 FilteringUDPMux

ICE 通过 `ice.WithNetworkTypes([]ice.NetworkType{ice.NetworkTypeUDP4})` 显式限定只使用 UDP4，v6conn 上不会出现任何 STUN 流量。因此：

- `makeReceiveIPv6` 可以安全地直接读取 v6conn，不存在与 connWorker 的竞争
- 无需为 v6conn 创建 `FilteringUDPMux` 或 passthrough channel
- 批量读取（`ReadBatch`）的性能优势得以保留

若未来启用 ICE over IPv6，必须为 v6conn 增加对称的 `FilteringUDPMux` 和 passthrough channel，否则会重现 IPv4 的竞争问题。

### 8.6 `passThroughCh` 满时丢弃

WireGuard 的消费速度（解密 goroutine）远快于网络包到达速率，channel 满的概率极低。满时丢弃优于阻塞 `readLoop`：阻塞 `readLoop` 会导致 STUN 包积压，ICE keepalive 超时，触发不必要的 ICE 重启。

---

## 9. 接口变更摘要

| 模块 | 变更项 | 说明 |
|------|--------|------|
| `internal/infra/ice.go` | `NewUdpMux` → `NewFilteringUDPMux` | 返回 Wrapper，包含 chanConn + inner mux |
| `internal/infra/mux_filter.go` | 新增 `FilteringUDPMux` | Wrapper 主体，唯一读取者，PassThrough 分叉 |
| `internal/infra/chan_conn.go` | 新增 `ChanPacketConn` | 给 UDPMuxDefault 用的假 PacketConn |
| `internal/infra/conn.go` | `makeReceiveIPv4/6` 从 `passThroughCh` 读取 | 移除 socket 直读和 `FilterMessage` 调用 |
| `internal/infra/conn.go` | `BindConfig` 重构 | 移除 `V4Conn/V6Conn/UniversalUDPMux`，新增 `FilteringMux/PassThrough` |
| `management/transport/agent.go` | `AgentWrapper` 移除 `RTieBreaker` | v4 无需手动比较 tiebreaker |
| `management/transport/ice_dialer.go` | `getAgent()` 改用 `NewAgentWithOptions` | functional options 风格 |
| `management/transport/ice_dialer.go` | `Dial()` 改用 `StartDial/StartAccept` + `AwaitConnect` | 角色由 `isInitiator` 决定 |
| `management/transport/ice_dialer.go` | 连接成功后异步关闭 `iceConn`（延迟 500ms） | WireGuard 接管穿透路径 |
| `management/transport/ice_dialer.go` | `ICETransport` 新增 `RemoteEndpoint` 字段 | 传递穿透地址给 onSuccess |

---

## 10. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| `passThroughCh` 满导致丢包 | WireGuard 包被丢弃 | channel 容量 512；消费方速度远快于到达速率；满时丢弃优于阻塞 readLoop |
| Agent 关闭后对端 ICE 超时 | 对端触发 ICE restart | 500ms 延迟保证最后几轮 check；WireGuard keepalive 维持 NAT |
| `ChanPacketConn` goroutine 泄漏 | 资源泄漏 | `done` channel 控制生命周期；`FilteringUDPMux.Close()` 同步等待 `readLoop` 退出 |
| `isInitiator` 与 ICE 角色不一致 | 双端都 StartDial 或都 StartAccept | `isInitiator` 基于 PeerID 大小，两端对同一对 peer 计算结果互为反值，不会产生相同角色 |
| 启用 ICE IPv6 但未加 v6 mux | v6conn 上 connWorker 与 makeReceiveIPv6 再次竞争 | 启用 `NetworkTypeUDP6` 时，必须同步为 v6conn 创建 `FilteringUDPMux` 并将 `makeReceiveIPv6` 改为从 channel 消费 |
