---
title: ICE Connection & LRP Relay
---

# ICE Connection & LRP Relay

> Source: `superpowers/specs/2026-05-01-ice-relay-wireguard-design.md`

## Motivation

Lattice connects nodes across arbitrary network topologies — home LANs, corporate firewalls, cloud VPCs. A WireGuard mesh alone fails behind symmetric NAT. The transport layer addresses this with a **dual-path strategy**: direct P2P via ICE when possible, relay fallback via LRP when NAT traversal fails.

Design priorities:
1. **Lowest latency wins**: ICE direct path preferred; LRP is the fallback
2. **Seamless upgrade**: LRP → ICE upgrade happens transparently mid-connection
3. **Single port**: WireGuard and ICE share UDP port 51820 via a filtering mux
4. **Multi-transport relay**: LRP supports both TCP (HTTP upgrade) and QUIC (datagram)

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
  │   FilteringUDPMux  │                  │   FilteringUDPMux  │
  │   :51820           │                  │   :51820           │
  └──┬─────────────┬──┘                  └──┬─────────────┬──┘
     │             │                        │             │
┌────▼────┐  ┌─────▼──────┐          ┌────▼────┐  ┌─────▼──────┐
│ ICE     │  │ LRP Client │          │ ICE     │  │ LRP Client │
│ Agent   │  │ (TCP/QUIC) │          │ Agent   │  │ (TCP/QUIC) │
└─────────┘  └─────┬──────┘          └─────────┘  └─────┬──────┘
                   └──────── Relay Server ────────────────┘

     Signaling: NATS (lattice.signals.peers.<PeerID>)
```

## ICE / NAT Traversal

Built on [pion/ice v4](https://github.com/pion/ice).

- STUN servers for reflexive address discovery
- TURN servers for relay candidates (fallback within ICE)
- `FilteringUDPMux` demultiplexes STUN and WireGuard packets on the same port

## LRP Relay

LRP (Lattice Relay Protocol) is the outer relay when ICE fails (e.g., symmetric NAT on both sides):

- TCP transport: HTTP-upgraded persistent connection
- QUIC transport: datagram-based, eliminates head-of-line blocking

## Connection State Machine

```
Created → Probing → ICEReady / LRPReady → Failed → Closed
```

ICE and LRP probe in parallel. First to succeed wins. The state machine handles upgrade (LRP → ICE) and fallback transparently.
