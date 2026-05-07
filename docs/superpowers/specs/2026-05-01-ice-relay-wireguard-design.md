# ICE, Relay & WireGuard Transport Design

> Status: Approved | 2026-05-01

## Motivation

Lattice connects nodes across arbitrary network topologies — home LANs, corporate firewalls, cloud VPCs. A WireGuard mesh alone fails behind symmetric NAT (common in enterprise and cellular networks). The transport layer addresses this with a dual-path strategy: direct P2P via ICE when possible, relay fallback via LRP when NAT traversal fails.

This is the foundation upon which all Lattice connectivity rests. Its design priorities:

1. **Lowest latency wins**: ICE direct path preferred; LRP is the fallback, not the default
2. **Seamless upgrade**: LRP → ICE upgrade happens transparently mid-connection
3. **Single port**: WireGuard and ICE share UDP port 51820 via a filtering mux
4. **Multi-transport relay**: LRP supports both TCP (HTTP upgrade) and QUIC (datagram)

## Architecture

```
                  Peer A                                  Peer B
              ┌───────────┐                          ┌───────────┐
   App ──────>│   wf0     │                          │   wf0     │<────── App
              │   TUN     │                          │   TUN     │
              └─────┬─────┘                          └─────┬─────┘
                    │                                      │
              ┌─────▼─────┐                          ┌─────▼─────┐
              │ WireGuard │                          │ WireGuard │
              │  Device   │                          │  Device   │
              └─────┬─────┘                          └─────┬─────┘
                    │                                      │
        ┌───────────▼───────────┐              ┌───────────▼───────────┐
        │   DefaultBind         │              │   DefaultBind         │
        │                       │              │                       │
        │  Send: ICE or LRP ────┼──────────────┼──> Send: ICE or LRP   │
        │  Recv: passthru + LRP │              │  Recv: passthru + LRP │
        └───┬───────────────┬───┘              └───┬───────────────┬───┘
            │               │                      │               │
     ┌──────▼──────┐  ┌─────▼──────┐       ┌──────▼──────┐  ┌─────▼──────┐
     │ Filtering   │  │ LRP Client │       │ Filtering   │  │ LRP Client │
     │ UDPMux      │  │ (TCP/QUIC) │       │ UDPMux      │  │ (TCP/QUIC) │
     │ :51820      │  └─────┬──────┘       │ :51820      │  └─────┬──────┘
     └──────┬──────┘        │              └──────┬──────┘        │
            │               │                     │               │
     ┌──────▼──────┐  ┌─────▼──────┐       ┌──────▼──────┐  ┌─────▼──────┐
     │ ICE Agent   │  │            │       │ ICE Agent   │  │            │
     │ (pion/ice)  │  │  ┌─────────▼──┐    │ (pion/ice)  │  │  ┌─────────▼──┐
     └─────────────┘  │  │  Relay     │    └─────────────┘  │  │  Relay     │
                      │  │  Server    │                      │  │  Server    │
                      └──┤            ├──────────────────────┘  │            │
                         └────────────┘                         └────────────┘

         Signaling: NATS (lattice.signals.peers.<PeerID>)
```

---

## 1. ICE / NAT Traversal

### Stack

Built on [pion/ice v4](https://github.com/pion/ice).

### Candidate Types

| Type | Support |
|------|---------|
| Host | Always |
| Server Reflexive (srflx) | Always (STUN: `stun.alattice.io:3478`) |
| Relay (TURN) | Pro only |

### FilteringUDPMux — Shared Socket

WireGuard and ICE share UDP port 51820. This creates a race — both want to read from the same socket. `FilteringUDPMux` solves this:

1. `readLoop` is the **sole reader** of the real UDP socket
2. Incoming packets classified:
   - **STUN** (detected via `stun.IsMessage`) → injected into `ChanPacketConn` → feeds pion/ice's mux
   - **Non-STUN** (WireGuard encrypted) → forwarded through `passThroughCh` → WireGuard `DefaultBind` receive path
3. Two instances: IPv4 and IPv6

### ICE Agent Configuration

- Interface filter: excludes virtual (docker, veth, br-, wf*)
- Uses `FilteringUDPMux.UDPMuxSrflx()` for server-reflexive candidates
- Disconnected timeout: 10s; Failed timeout: 15s
- `OnConnectionStateChange`: Failed → triggers `Close()` → returns `ErrDialerClosed`
- `OnCandidate`: each gathered candidate sent as `OFFER` signaling packet via NATS

### Candidate Exchange

```
Initiator (higher PeerID numeric)
  │  HANDSHAKE_SYN (every 2s)
  ├──────────────────────────────>
  │                               Responder
  │                               │  HANDSHAKE_ACK
  │  <────────────────────────────┤
  │  OFFER (ICE candidate)        │
  ├──────────────────────────────>│  OFFER (ICE candidate)
  │                               ├──────────────────────────────>
  │  ANSWER (ICE candidate)       │
  <───────────────────────────────┤
  │        ICE Connected          │
```

1. Initiator (higher `PeerID.ToUint64()`) sends `HANDSHAKE_SYN` via NATS every 2s
2. Responder receives SYN, sends `HANDSHAKE_ACK`, creates ICE agent, begins `GatherCandidates()`
3. Initiator receives ACK, cancels SYN timer, begins `GatherCandidates()`
4. Candidates exchanged via `OFFER`/`ANSWER` signaling packets
5. `Dial()` unblocks on first candidate → `StartDial()`/`StartAccept()` → `AwaitConnect()`

---

## 2. LRP (Lattice Relay Protocol)

### Protocol Frame (12 bytes, little-endian)

```
┌──────────────┬──────────────┬──────┬──────────────┬──────────┐
│ Seq (2B)     │ PayloadLen   │ Cmd  │ ToID (4B)    │ Reserved │
│ uint16       │ (4B) uint32  │ (1B) │ uint32       │ (1B)     │
└──────────────┴──────────────┴──────┴──────────────┴──────────┘
```

**Commands:**

| Cmd | Name | Purpose |
|-----|------|---------|
| `0x01` | Register | Session registration (fromId → relay) |
| `0x02` | Forward | Relay WireGuard payload to target peer |
| `0x03` | KeepAlive | Reset 30s read deadline |
| `0x04` | Probe | Connectivity probe to target peer |

### Server Architecture

Two transport modes run alongside, sharing the same `SessionManager`:

**TCP server** (`lrp_server.go`):
1. HTTP listener → client performs `Upgrade: bolt` handshake
2. Server hijacks connection → reads Register frame → registers session
3. Main loop: KeepAlive (reset deadline) | Forward/Probe (relay via `SessionManager`)

**QUIC server** (`lrp_server_quic.go`):
1. `quic.ListenAddr` with self-signed TLS (ALPN: "lrp")
2. Accept QUIC connection → read control stream → Register frame
3. `relayDatagrams` goroutine: Forward/Probe via QUIC datagrams
4. `handleControlStream`: KeepAlive on control stream

### Client Architecture

**TCP client** (`lrp_client_tcp.go`):
- TCP connect → HTTP Upgrade (`Upgrade: lrp`) → Register frame
- `writerLoop` goroutine: coalesced writes from `sendCh`
- `keepaliveLoop`: KeepAlive every 20s
- `ReceiveFunc()`: reads LRP frames, returns Forward as WireGuard packets with `LRPEndpoint{TransportType: LRP}`

**QUIC client** (`lrp_client_quic.go`):
- QUIC dial with `EnableDatagrams: true`
- Control stream for registration
- `Send()`: frames as QUIC datagrams
- `ReceiveFunc()`: reads QUIC datagrams, classifies Forward/Probe

### Session Manager

Central session registry for relay forwarding:
- Registers each connected client as `fromId → stream/conn`
- `Relay(fromId, targetId, payload)`: looks up target session, forwards frame
- Shared across TCP and QUIC servers

---

## 3. WireGuard Integration

### DefaultBind (WireGuard `conn.Bind`)

The critical integration point — implements WireGuard's binding interface:

**Receive path** (`Open()`):
- Returns `ReceiveFunc` handlers reading from two sources:
  1. `passThroughCh` — non-STUN packets from FilteringUDPMux (ICE/direct path)
  2. LRP client's `ReceiveFunc()` — relayed packets
- All received packets wrapped in `LRPEndpoint`

**Send path** (`Send()`):
- Checks `LRPEndpoint.TransportType`:
  - `ICE` → standard UDP socket (`WriteBatch` on Linux, `WriteMsgUDP` otherwise)
  - `LRP` → `lrperClient.Send()` through relay

### LRPEndpoint (WireGuard `conn.Endpoint`)

Unified endpoint carrying both physical and relay metadata:

| Method | ICE path | LRP path |
|--------|----------|----------|
| `DstToBytes()` | Marshaled `netip.AddrPort` | Encoded `RemoteId` bytes |
| `DstIP()` | Real IP | `IPv4Unspecified` (ID-based routing) |
| `DstToString()` | IP:port string | Relay address string |

Fake IPv6 prefix (`fd6c:7270::`) encodes RemoteId for `wg show` compatibility.

### WireGuard UAPI Handler

`DeviceManager` runs a UAPI socket handler for standard WireGuard config operations (`set=1`, `get=1`), delegating directly to the `wg.Device` from the official library.

---

## 4. Transport State Machine

### States

```
Created → Probing → ICEReady / LRPReady → Failed → Closed
                   ↖                         ↓        ↗
                     └───────────────────────┘
```

### Allowed Transitions

| From | To |
|------|-----|
| Created | Probing |
| Probing | ICEReady, LRPReady, Failed |
| LRPReady | ICEReady (upgrade), Failed, Closed |
| ICEReady | Failed, Closed |
| Failed | Probing (restart), Closed |

### Probe — Lifecycle Orchestrator

Each remote peer gets a `Probe` that manages connection lifecycle:

1. **`Start()`**: CAS on `running` flag → transition `Created → Probing` → launch `discover()`
2. **`discover()`**: Races ICE and LRP dialers concurrently
   - Both call `Prepare()` (SYN every 2s, up to 60s) then `Dial()` (wait for candidate exchange)
   - **Race priority**: ICE > LRP (lower latency P2P preferred)
   - If LRP wins first → wait 500ms for ICE → if ICE arrives, discard LRP; otherwise use LRP and continue ICE in background
3. **`onSuccess()`**: Transition to `ICEReady` or `LRPReady`
4. **`handleUpgradeTransport()`**: LRP → ICE upgrade
   - Replace `currentTransport` with ICE transport
   - Close old LRP transport after 2s delay
   - Transition `LRPReady → ICEReady`
5. **`onFailure()`**:
   - `ErrDialerClosed`: clean session end → restart immediately
   - Other errors: track `firstFailureAt` → 60s unreachable → `Closed`; otherwise `Failed` → restart after 10s
6. **`restart()`**: Create fresh dialers, increment epoch (invalidates old goroutines), transition `Failed → Probing`

### Connection Configurator Side Effects

On state transitions, the `Configurator` applies WireGuard changes:

```
Probing → ICEReady/LRPReady:
  - SetEndpoint (WireGuard peer endpoint)
  - ApplyRoute (kernel route for peer VPN IP)
  - SetupNAT (NAT rules for wf0 interface)

LRPReady → ICEReady (upgrade):
  - SetEndpoint only (no duplicate peer/route/NAT)

→ Failed/Closed:
  - RemovePeer (clean up WireGuard state)
```

### Metrics

Exposed via VictoriaMetrics:
```
lattice_transport_state_changes_total{from, to}
```

---

## 5. Signaling (NATS)

### Subjects

```
lattice.signals.peers.<PeerID>
```

PeerID: first 8 bytes of WireGuard public key, encoded as uint64.

### Protobuf Signal Packet

```protobuf
message SignalPacket {
    PacketType type;     // HANDSHAKE_SYN, HANDSHAKE_ACK, OFFER, ANSWER, MESSAGE
    DialerType dialer;   // ICE or LRP
    uint64 sender_id;
    oneof payload {
        Handshake handshake;  // peer info (JSON)
        Offer offer;          // ICE candidate or LRP offer
        Offer answer;         // ICE candidate or LRP answer
        Message message;      // config push from control plane
    }
}
```

### Flow

1. Node subscribes to its own subject at startup
2. Control plane pushes config via `MESSAGE` packets → `MessageHandler` processes → `AddPeer` → creates `Probe`
3. Peer-to-peer signaling via `Publish`/`RequestWithContext`:
   - `RequestWithContext` uses exponential backoff: 100ms → 200ms → 400ms → 800ms → 1600ms (5 retries)
4. On NATS reconnect: re-register, re-apply network map

### Initiator Determination

```go
isInitiator = local.ID().ToUint64() > remote.ID().ToUint64()
```

Numeric comparison of uint64 PeerID — deterministic, no additional negotiation needed.

---

## 6. End-to-End Datapath

### Data (Encrypted Traffic)

```
App on A → wf0 TUN → WireGuard encrypt → DefaultBind.Send()
  ├── ICE path: FilteringUDPMux → UDP :51820 ────────────────> Peer B
  └── LRP path: LRP Forward frame → Relay Server → Peer B

Peer B: DefaultBind.ReceiveFunc()
  ├── ICE path: passThroughCh ← FilteringUDPMux ← UDP :51820
  └── LRP path: LRP client ReceiveFunc ← Relay Server
  → WireGuard decrypt → wf0 TUN → App on B
```

### Signaling (Connection Establishment)

```
1. Control plane → MESSAGE (NATS) → MessageHandler → AddPeer → Probe(StateCreated)
2. Probe.Start() → Probing → discover()
3. HANDSHAKE_SYN/ACK exchange via NATS (2s interval)
4. OFFER/ANSWER candidate exchange via NATS
5. Transport established → ICEReady/LRPReady
6. onSuccess() → WireGuard SetEndpoint + ApplyRoute + SetupNAT
7. [If LRP] → background ICE → upgrade to ICEReady when ready
```

---

## 7. Pro/Community Split

| Feature | Community | Pro |
|---------|-----------|-----|
| ICE (host + srflx) | Yes | Yes |
| LRP relay (TCP + QUIC) | Yes | Yes |
| TURN relay | 402 stub | Yes |
| State machine metrics | Yes | Yes |

---

## File Layout

```
internal/agent/infra/ice.go                 ICE agent factory, filtering mux
internal/agent/infra/mux_filter.go          FilteringUDPMux implementation
internal/agent/infra/chan_conn.go           ChanPacketConn (STUN injection)
internal/agent/infra/conn.go                DefaultBind (WireGuard conn.Bind)
internal/agent/infra/wrrp.go                LRPEndpoint (WireGuard conn.Endpoint)
internal/agent/infra/transport.go           SignalService + Probe interfaces
internal/agent/infra/peer.go                PeerID, PeerIdentity
internal/agent/infra/device_conf.go         WireGuard DeviceConf
internal/agent/wireguard/wg.go              DeviceManager, UAPI handler (Unix)
internal/agent/wireguard/wg_windows.go      DeviceManager (Windows)
internal/agent/wireguard/status.go          WireGuard status reporting
internal/agent/wireguard/handshake_watcher.go  First handshake watcher
internal/server/transport/ice_dialer.go     ICE Dialer (pion/ice agent)
internal/server/transport/lrp_dialer.go     LRP Dialer (relay negotiation)
internal/server/transport/state_machine.go  Transport state machine
internal/server/transport/state.go          ConnectionState enum
internal/server/transport/probe.go          Probe lifecycle orchestrator
internal/server/transport/probe_factory.go  ProbeFactory (all probes)
internal/relay/lrp_protocol.go             LRP frame definition (12 bytes)
internal/relay/lrp_server.go               TCP relay server (HTTP upgrade)
internal/relay/lrp_server_quic.go          QUIC relay server
internal/relay/lrp_client_tcp.go           TCP relay client
internal/relay/lrp_client_quic.go          QUIC relay client
internal/relay/lrp_client.go               Shared relay client logic
internal/relay/session_manager.go          Session registration + forwarding
internal/relay/conn.go                     ReadWriterConn wrapper
internal/relay/pool.go                     Buffer pools
internal/relay/stream.go                   Stream/Session interfaces
internal/relay/turn_server.go              TURN server (Pro)
internal/relay/turn_client.go              TURN client (Pro)
internal/relay/turn_community.go           TURN community stub
internal/server/nats/nats.go               NATS signal service
internal/grpc/signal.pb.go                 Protobuf signal definitions
internal/agent/tunnel/drp.go               DRP frame types (legacy)
internal/agent/tunnel/endpoint.go          MagicEndpoint
```
