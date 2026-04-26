# ICE/WRRP Transport Layer Redesign

## Background

Wireflow nodes establish encrypted WireGuard tunnels through a combination of ICE
hole-punching and WRRP relay fallback.  The signaling protocol uses a SYN/ACK
handshake followed by an ICE candidate/credential exchange (OFFER/ANSWER).

This document describes eight design defects found in the original implementation
and the redesigned architecture that resolves them.

---

## Problems with the Original Design

### P1 — Peer info piggybacked onto ICE OFFER (timing dependency)

WireGuard peer config (VPN address, AllowedIPs) was embedded inside `Offer.current`
(the ICE candidate packet).  The ICE agent must exist and have gathered at least one
candidate before the first OFFER can be sent.  This creates a hard dependency:

```
WG config only known after first ICE candidate gathered
  → WG AddPeer called inside onSuccess
  → onSuccess fires after transport established (seconds later)
```

The remote peer's WG config should be known as early as possible — ideally before
any transport negotiation begins.

### P2 — `localPeer` stale capture at probe creation time

`NewProbe()` captured the local peer's `*infra.Peer` snapshot at probe creation.
If a remote SYN arrived before `ApplyFullConfig` completed (e.g. during startup),
the captured peer had `Address == nil` and `AllowedIPs == ""`.  All subsequent
dialers for that probe then sent null peer info in OFFERs, causing the remote's
`onSuccess` to hit a nil-dereference or configure a broken WG route.

### P3 — `IsCredentialsInited` race in OFFER handler

```
G1 (OFFER handler): IsCredentialsInited = false → enters block → starts JSON unmarshal
G2 (OFFER handler): IsCredentialsInited = false → enters block (concurrent)
G2: finishes first → sets remotePeer → Store(true)
G1: finishes → sets remotePeer (overwrite) → Store(true) already done
```

Or worse: `Store(true)` happened before `onPeerReceived()`, allowing a concurrent
handler to skip the credentials block entirely while `remotePeer` was still nil.

### P4 — Inconsistent role determination (three different comparisons)

| Location | Comparison | Type |
|---|---|---|
| `iceDialer.Prepare()` | `localId.String() < remoteId.String()` | Lexicographic string |
| `probe_factory.onSuccess` | `localId.ID().ToUint64() > remoteId.ID().ToUint64()` | Numeric uint64 |
| `wrrpDialer.Handle(ACK)` | `localId.String() > remoteId.String()` | Lexicographic string |

String comparison of decimal uint64 produces incorrect results: `"9" > "14"` is
`true` lexicographically but `9 < 14` numerically.

### P5 — ICE and WRRP share packet types but follow different protocols

Both dialers handle `HANDSHAKE_SYN` / `HANDSHAKE_ACK` / `OFFER` / `ANSWER` but with
different semantics.  WRRP uses OFFER/ANSWER as its own readiness signal (carrying
peer info in `Offer.current`), while ICE uses OFFER/ANSWER exclusively for candidate
exchange.  The `DialerType` field was added as a workaround to demultiplex packets,
but the protocol structure remains confusing.

### P6 — `onSuccess` couples WG config with transport establishment

`onSuccess` calls `AddPeer + ApplyRoute + SetupNAT` atomically after transport is
established.  This means:

- WG cannot pre-configure the peer (no peer until transport ready).
- If `onSuccess` fails (e.g. provisioner not yet ready), the transport is discarded
  and must be fully re-established.
- `handleUpgradeTransport` re-invokes `onSuccess` for the ICE→WRRP upgrade path,
  risking duplicate `AddPeer` / `ApplyRoute` calls.

### P7 — `restart()` does not clean up stale WG state

`Probe.restart()` replaces dialers and re-runs `Start()`, but never calls
`provisioner.RemovePeer()` to clear the old WireGuard peer entry.  If the peer's
endpoint or allowed IPs changed, the stale entry persists until `AddPeer` overwrites
it (which may not happen until after `onSuccess` fires again).

### P8 — `probe.started` atomic cannot distinguish "running" from "stopped"

`started` is set to `false` inside `restart()` synchronously, but the old `discover()`
goroutine may still be in flight.  Two goroutines can both reach `onSuccess` for the
same probe.

---

## New Architecture

### Core Principle: Peer info travels in SYN/ACK, not OFFER

WG peer config is piggybacked onto the earliest possible signal message — the
handshake SYN and ACK — so that both sides know the peer's VPN address and public
key **before** any ICE candidate gathering begins.

```
Initiator (local > remote numerically)      Responder
        │                                        │
        │── SYN { peer_info: A } ──────────────►│
        │                           onPeerKnown(A) → AddPeer + ApplyRoute
        │◄─ ACK { peer_info: B } ───────────────│
onPeerKnown(B) → AddPeer + ApplyRoute
        │                                        │
        │  [GatherCandidates]        [GatherCandidates]
        │                                        │
        │── OFFER { ufrag,pwd,cand } ───────────►│
        │◄─ OFFER { ufrag,pwd,cand } ────────────│
        │                                        │
        │        [ICE connectivity checks]       │
        │                                        │
        │◄──────── ICE Connected ───────────────►│
        │                                        │
  onEndpointReady → SetPeer(endpoint) + SetupNAT │
```

### Protocol Changes

#### `signal.proto`: `Handshake.peer_info`

```protobuf
message Handshake {
  int64 timestamp = 1;
  bytes peer_info = 2;  // JSON-encoded infra.Peer (local WG config)
}
```

`Offer.current` is kept for backward compatibility but is no longer authoritative:
`onPeerReceived` is idempotent and only updates the peer manager on the first call.

### Unified Role Determination: `isInitiator()`

Replace all three inconsistent comparisons with a single function:

```go
// isInitiator returns true when the local node should drive the ICE handshake.
// Uses numeric uint64 comparison to avoid decimal string ordering bugs
// ("9" > "14" lexicographically but 9 < 14 numerically).
func isInitiator(local, remote infra.PeerIdentity) bool {
    return local.ID().ToUint64() > remote.ID().ToUint64()
}
```

### `GetLocalPeer` as a Lazy Function

Both dialers receive `GetLocalPeer func() *infra.Peer` instead of a captured
`*infra.Peer` snapshot.  The function is called at **send time** so that a
late-arriving `ApplyFullConfig` (which sets `Address` and `AllowedIPs`) is always
reflected in SYN/ACK peer info.

### Two-Step WG Configuration

| Event | Action |
|---|---|
| `onPeerKnown(peer)` — first SYN/ACK received | `AddPeer` (public key + AllowedIPs) + `ApplyRoute` |
| `onEndpointReady(transport)` — ICE/WRRP connected | `SetPeer(endpoint)` + `SetupNAT` |

This decouples route programming from endpoint discovery.  `AddPeer` can be called
as soon as the peer's identity is known; `SetPeer(endpoint)` updates the WireGuard
peer entry with the actual UDP endpoint once the tunnel is established.

### Restart Cleanup

`Probe.restart()` calls `provisioner.RemovePeer(publicKey)` before rebuilding dialers,
ensuring stale WG state is cleared on every session reset.

---

## Implementation Status

| Priority | Change | Files | Status |
|---|---|---|---|
| P1 | Add `peer_info` to `Handshake` proto | `signal.proto`, regenerate | ✅ done |
| P1 | Add `isInitiator()` helper | `transport/role.go` (new) | ✅ done |
| P1 | Pass `GetLocalPeer func()` to dialers | `ice_dialer.go`, `wrrp_dialer.go`, `probe_factory.go` | ✅ done |
| P1 | Include `peer_info` in SYN/ACK send; call `onPeerReceived` in SYN/ACK handle | `ice_dialer.go`, `wrrp_dialer.go` | ✅ done |
| P1 | Fix role comparison in `wrrpDialer.Handle(ACK)` and `iceDialer.Prepare()` | `ice_dialer.go`, `wrrp_dialer.go` | ✅ done |
| P2 | Split `onSuccess` into `onPeerKnown` + `onEndpointReady` | `probe_factory.go` | ✅ done |
| P2 | Call `RemovePeer` in `restart()` via `onBeforeRestart` callback | `probe.go`, `probe_factory.go` | ✅ done |
| P3 | Epoch-based state machine replacing `started atomic.Bool` | `probe.go` | ✅ done |

### P3 Implementation Notes

`started atomic.Bool` was replaced with two fields:

- `epoch atomic.Uint64` — incremented on every `restart()`.  A `discover()` goroutine
  captures `myEpoch` at launch and checks `p.epoch.Load() != myEpoch` before calling
  `onSuccess`/`onFailure`.  If the epoch changed, the goroutine discards its result
  and exits — the new goroutine spawned by `restart()` is the sole owner.
- `running atomic.Bool` — guarded by `CompareAndSwap(false, true)` in `Start()`,
  preventing concurrent `discover()` goroutines.  Reset by `restart()` (before the
  new `Start()`) and by the goroutine itself on normal completion.

This eliminates the race where `restart()` called `started.Store(false)` while the old
goroutine was still in flight, allowing two goroutines to both reach `onSuccess`.
