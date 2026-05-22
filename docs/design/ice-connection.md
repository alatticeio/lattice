---
title: ICE Connection & LRP Relay
---

# ICE Connection & LRP Relay

> Source: `superpowers/specs/2026-05-01-ice-relay-wireguard-design.md`, `2026-05-18-ice-handshake-design.md`

## Overview

Lattice establishes peer-to-peer WireGuard tunnels using a **dual-path race**: ICE (direct UDP) and LRP (relay fallback). NATS carries signaling messages. Both share UDP `:51820` via `FilteringUDPMux`.

## Architecture

```
          Peer A                                  Peer B
      ┌───────────┐                          ┌───────────┐
      │   wf0     │                          │   wf0     │
      │   TUN     │                          │   TUN     │
      └─────┬─────┘                          └─────┬─────┘
            │                                      │
      ┌─────▼─────┐                          ┌─────▼─────┐
      │ WireGuard │                          │ WireGuard │
      │  Device   │                          │  Device   │
      └─────┬─────┘                          └─────┬─────┘
            │                                      │
  ┌─────────▼─────────┐                  ┌─────────▼─────────┐
  │  FilteringUDPMux   │                  │  FilteringUDPMux   │
  │  :51820            │                  │  :51820            │
  └──┬─────────────┬──┘                  └──┬─────────────┬──┘
     │             │                        │             │
┌────▼────┐  ┌─────▼──────┐          ┌────▼────┐  ┌─────▼──────┐
│ ICE     │  │ LRP Client │          │ ICE     │  │ LRP Client │
│ Agent   │  │ (QUIC/TCP) │          │ Agent   │  │ (QUIC/TCP) │
└─────────┘  └─────┬──────┘          └─────────┘  └─────┬──────┘
                   └──────── Relay Server ────────────────┘

     Signaling: NATS (lattice.signals.peers.<PeerID>)
```

## ICE Handshake (4-message via NATS)

Initiator is determined by `localId > remoteId` (numeric uint64 comparison).

```
Initiator (larger ID)                   Responder (smaller ID)
────────────────────────                ───────────────────────
Prepare():
  getAgent() + GatherCandidates()       Prepare():
  send SYN ──────────────────►            (waits for SYN)

                                        Handle(SYN):
  ◄─────────────────── ACK               1. extract peer_info
                                         2. getAgent()
Handle(ACK):                             3. send ACK
  GatherCandidates() (if needed)
  OFFER(candidate) ──────────►          Handle(OFFER):
  ◄────────── ANSWER(candidate)           AddRemoteCandidate()

Dial():                                 Dial():
  AwaitConnect()                          AwaitConnect()
  ◄══════ ICE Connected ══════►          ◄══════ ICE Connected ══════►
```

- **SYN retry**: every 2s via ticker, stops when ACK received
- **SYN + active agent**: responder resends ACK + candidates
- **SYN + closed agent**: responder triggers `restart()` (remote restarted)

## Concurrent Race & LRP Fallback

`Probe.discover()` runs both dialers concurrently:

```
ICE goroutine ─┐
               ├──► result ← first transport to succeed
LRP goroutine ─┘
```

| Outcome | Behavior |
|---------|----------|
| ICE wins | Return ICE transport immediately |
| LRP wins, ICE arrives within 500ms | Close LRP, return ICE |
| LRP wins, ICE arrives after 500ms | Keep LRP, ICE upgrades later |
| Both fail | Record failure, retry in 10s or close after 60s |

## Connection State Machine

```
Created → Probing → ICEReady ─┬→ Failed → Probing (10s retry)
                 → LRPReady ─┤         → Closed (60s, permanent)
                              │
                 LRPReady → ICEReady (transparent upgrade)
```

| State | Meaning |
|-------|---------|
| `created` | Probe allocated |
| `probing` | ICE + LRP racing |
| `ice-ready` | ICE direct connected |
| `lrp-ready` | LRP relay active |
| `failed` | All dialers failed |
| `closed` | Unreachable for 60s |

Callbacks: `Probing→Ready` sets WireGuard endpoint + route + NAT; `LRPReady→ICEReady` only updates endpoint; `→Failed/Closed` removes peer.

## FilteringUDPMux

ICE STUN and WireGuard share UDP `:51820`. Two goroutines read the same `net.UDPConn`:

| Packet | ICE agent | WireGuard |
|--------|-----------|-----------|
| STUN (magic `0x2112A442`) | ✅ dispatched | ❌ decrypt fails |
| WireGuard encrypted | ❌ no ufrag match | ✅ dispatched |

`FilteringUDPMux` inspects bytes 4-7 for the STUN magic cookie and routes accordingly. After ICE succeeds, the agent closes and WG `PersistentKeepalive` maintains NAT.

## Key Files

| File | Role |
|------|------|
| `internal/server/transport/state_machine.go` | State machine |
| `internal/server/transport/ice_dialer.go` | ICE dialer (SYN/ACK/OFFER/ANSWER) |
| `internal/server/transport/probe.go` | Per-peer probe, discover() race |
| `internal/server/transport/probe_factory.go` | Probe lifecycle, callbacks |
| `internal/server/transport/lrp_dialer.go` | LRP relay dialer (QUIC+TCP) |
| `internal/agent/infra/mux_filter.go` | FilteringUDPMux |
