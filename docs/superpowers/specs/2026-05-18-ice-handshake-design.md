# ICE Handshake & Transport Design

> Generated from code: `internal/server/transport/` (2026-05-18)

## Overview

Lattice establishes peer-to-peer WireGuard tunnels by racing two transport dialers concurrently: **ICE** (direct UDP hole-punching) and **LRP** (QUIC relay fallback). The faster transport wins and becomes the WireGuard endpoint; if ICE later succeeds after LRP won, it transparently upgrades the connection.

## Key Files

| File | Responsibility |
|------|---------------|
| `ice_dialer.go` | ICE handshake state machine, agent lifecycle |
| `lrp_dialer.go` | QUIC relay dialer |
| `probe.go` | Per-peer probe: races dialers, manages lifecycle |
| `probe_factory.go` | Creates/caches probes, wires WireGuard callbacks |
| `state_machine.go` | Peer connection state machine |
| `wg_configurator.go` | Serializes WireGuard peer/route/NAT operations |

---

## Peer State Machine

```
Created → Probing → ICEReady ─┐
                └→ LRPReady → ICEReady (upgrade)
                              │
Probing/ICEReady/LRPReady → Failed → Probing (retry)
                                   → Closed (permanent)
```

### States

| State | Meaning |
|-------|---------|
| `Created` | Probe allocated, not yet started |
| `Probing` | Both dialers racing |
| `ICEReady` | ICE transport active, WireGuard endpoint set |
| `LRPReady` | Relay transport active, WireGuard fake endpoint set |
| `Failed` | All dialers failed; 10s retry or immediate restart |
| `Closed` | Peer unreachable for 60s; WireGuard peer removed |

### Transition Callbacks (in `probe_factory.go`)

- **`Probing → ICEReady/LRPReady`**: `SetEndpoint` + `ApplyRoute` + `SetupNAT`
- **`LRPReady → ICEReady`** (upgrade): `SetEndpoint` only — no duplicate `AddPeer`/route/NAT
- **`→ Failed` or `→ Closed`**: `RemovePeer` from WireGuard

---

## ICE Handshake Protocol

### Role Determination

```go
func isInitiator(localId, remoteId infra.PeerIdentity) bool {
    return localId.ID().ToUint64() > remoteId.ID().ToUint64()
}
```

The peer with the numerically larger `PeerID` is the **initiator**. This is deterministic and symmetric — both sides agree on roles without coordination.

### Message Flow

```
Initiator                          Responder
    │                                  │
    │──── RESTART_NOTIFY ─────────────►│  (non-init notifies it's fresh)
    │                                  │  (create ICE agent)
    │──── SYN (+ local peer info) ────►│  (retried every 2s, up to 60s)
    │                                  │  (create ICE agent, set i.agent)
    │◄─── ACK (+ local peer info) ─────│  (ACK sent AFTER agent ready)
    │    cancel SYN ticker             │
    │    GatherCandidates()            │  GatherCandidates()
    │──── OFFER (candidate) ──────────►│
    │◄─── OFFER (candidate) ───────────│
    │    (exchange continues...)       │
    │    StartDial(rUfrag, rPwd)       │  StartAccept(rUfrag, rPwd)
    │    AwaitConnect()                │  AwaitConnect()
    │         ICE Connected            │
    │◄════ WireGuard tunnel ══════════►│
```

### Critical Ordering: ACK After Agent Init

**The responder MUST initialize `i.agent` before sending ACK.**

Upon receiving ACK, the initiator immediately calls `GatherCandidates()` and starts sending OFFERs. These can arrive within 1–2 ms. If ACK is sent before `i.agent` is set, OFFERs are silently dropped:

```go
// Handle() OFFER case:
i.mu.Lock()
agent := i.agent
i.mu.Unlock()
if agent == nil {
    i.log.Debug("receive offer: agent nil, dropping", ...)
    return nil  // OFFER lost → ICE stalls
}
```

**Correct order in SYN handler:**
```go
// 1. Create agent
agent, err := i.getAgent(remoteId)
// 2. Set i.agent under mutex
i.mu.Lock()
i.agent = agent
i.mu.Unlock()
// 3. Only then send ACK
i.sendPacket(ctx, ..., grpc.PacketType_HANDSHAKE_ACK, nil)
// 4. Responder gathers too
agent.GatherCandidates()
```

### SYN Retransmit Handling

SYN is sent every 2s by the initiator. When a SYN arrives while ICE setup is already in progress (`existingAgent != nil`), the responder **resends ACK** instead of creating a new agent. This prevents destroying ongoing candidate exchange:

```go
if existingAgent != nil {
    // SYN retransmit — just ACK again
    _ = i.sendPacket(ctx, ..., grpc.PacketType_HANDSHAKE_ACK, nil)
    return nil
}
```

### Peer Info Exchange

Peer metadata (WireGuard address, AllowedIPs) is piggybacked on SYN and ACK packets (`hs.PeerInfo`). This allows the WireGuard peer entry to be pre-configured (`onPeerKnown`) before ICE candidate exchange completes, reducing connection setup latency.

OFFERs also carry `Current` (peer info) for backward compatibility with older nodes.

---

## Mutex Discipline (`iceDialer`)

`i.agent` is accessed from multiple goroutines: the NATS message handler (Handle), the OnCandidate callback, and Close. **All reads and writes must be under `i.mu`.**

| Field | Protection |
|-------|-----------|
| `i.agent` | `i.mu` (Mutex) |
| `i.rUfrag`, `i.rPwd` | `i.mu` (double-checked locking) |
| `i.cancel` | `i.mu` |
| `i.credentialsInited` | `atomic.Bool` |
| `i.closed` | `atomic.Bool` |

### Close Sequence

```go
func (i *iceDialer) Close() error {
    i.closeOnce.Do(func() {
        i.closed.Store(true)      // 1. gate all incoming packets
        i.mu.Lock()
        agent := i.agent
        i.agent = nil             // 2. clear under mutex
        i.mu.Unlock()
        close(i.closeChan)        // 3. unblock Dial() → returns ErrDialerClosed
        agent.Close()             // 4. teardown ICE agent
    })
}
```

`ErrDialerClosed` from `Dial()` signals `onFailure` to do an immediate `probe.restart()` (no 10s delay), since this is a clean session transition, not a network error.

---

## `discover()`: Racing Dialers

`Probe.discover()` runs ICE and LRP dialers concurrently:

```go
// Dialers captured under read-lock BEFORE goroutines spawn.
// This prevents restart() from swapping p.iceDialer between
// Prepare() and Dial() calls in the goroutine.
p.mu.RLock()
iceD := p.iceDialer
lrpD := p.lrpDialer
p.mu.RUnlock()

go func() { iceD.Prepare(...); iceD.Dial(...) }()
go func() { lrpD.Prepare(...); lrpD.Dial(...) }()
```

**Winner selection**: If LRP wins first, `discover()` waits 500ms for ICE. If ICE arrives within that window, LRP is discarded and ICE is used. If not, LRP becomes the active transport and ICE upgrades later via `handleUpgradeTransport`.

---

## Probe Lifecycle

### Epoch Counter

Each `restart()` increments `p.epoch`. After `discover()` completes, it checks if the epoch changed. Stale results (from a previous epoch) are discarded:

```go
if p.epoch.Load() != myEpoch {
    t.Close()
    return
}
```

### Failure Handling

| Error | Action |
|-------|--------|
| `ErrDialerClosed` | Immediate `restart()` |
| Other error, elapsed < 60s | 10s `time.AfterFunc(restart)` |
| Other error, elapsed ≥ 60s | Transition to `Closed` |

### Restart Sequence

```
restart() {
    1. restartInProgress.CompareAndSwap(false, true)  // deduplicate
    2. onBeforeRestart()  // reset peerKnownDone flag
    3. p.mu.Lock(); p.iceDialer = newIceDialer(); p.lrpDialer = newLrpDialer()
    4. epoch.Add(1)
    5. sm.Transition(StateFailed)  // ensure clean state
    6. p.running.Store(false)
    7. p.Start(ctx, remoteId)      // begin new discover() goroutine
}
```

---

## WireGuard Configuration Serialization

All WireGuard operations go through `WGConfigurator` (not direct provisioner calls). This serializes `AddPeer`, `SetEndpoint`, `RemovePeer`, `ApplyRoute`, `SetupNAT` calls to prevent races when ICE and LRP complete simultaneously or when transitions fire close together.

`onPeerKnown` (first SYN/ACK with peer info) pre-registers the WireGuard peer entry. The state machine callback (`Probing → ICEReady/LRPReady`) then calls `SetEndpoint` with the actual UDP address, making the tunnel active.

---

## ICE Agent Configuration

```go
ice.WithDisconnectedTimeout(10 * time.Second)
ice.WithFailedTimeout(15 * time.Second)
```

- **Disconnected (not Failed)**: ICE retries keepalives aggressively. `Close()` is NOT triggered on Disconnected — only on Failed.
- **CandidateTypes**: Host + ServerReflexive (no TURN relay in ICE layer; LRP dialer handles relay fallback at application layer)
- **Interface filter**: Excludes docker, veth, br-, and `wf*` (WireGuard TUN) to prevent routing loops

### Dial Timeout

`Dial()` uses a 65s timeout (SYN window is 60s + margin). If no OFFER arrives before the initiator stops retrying, `Dial()` returns error → `onFailure()` → `probe.restart()`, preventing permanent deadlock.
