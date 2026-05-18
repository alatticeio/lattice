---
title: WRRP / QUIC Relay Transport
---

# WRRP / QUIC Relay Transport

> Source: `superpowers/specs/design-wrrp-quic.md`

## Background

WRRP (Wireflow Relay & Routing Protocol) is the relay channel used when two peers cannot establish a direct ICE path. The original implementation tunnels WireGuard packets over a persistent HTTP-upgraded TCP connection.

### Problems with TCP relay

| Problem | Impact |
|---------|--------|
| Head-of-Line blocking | A dropped TCP segment stalls all peers on the same connection |
| Application-level keepalive | Custom Ping/Pong logic required |
| Slow reconnect | TCP + HTTP handshake on every reconnect |
| No packet-boundary preservation | Framing layer required on top of TCP stream |

## Goals

1. Eliminate HoL blocking between peer pairs sharing a relay connection
2. Simplify keepalive — delegate to QUIC's built-in mechanism
3. Faster reconnect — QUIC 0-RTT resumption
4. Preserve packet boundaries — QUIC datagrams map 1:1 to WireGuard packets
5. Backward compatibility — TCP relay path remains fully operational; QUIC is opt-in

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    WRRP Relay Server                     │
│                                                          │
│  ┌─────────────────────┐   ┌─────────────────────────┐  │
│  │   TCP Server        │   │   QUIC Server           │  │
│  │   :6266 (HTTP upg.) │   │   :6267 (UDP)           │  │
│  └────────┬────────────┘   └──────────┬──────────────┘  │
│           └──────────┐   ┌────────────┘                  │
│                      ▼   ▼                               │
│               ┌─────────────────┐                        │
│               │  WRRPManager    │                        │
│               │  streams map    │  ← TCP sessions        │
│               │  quicConns map  │  ← QUIC sessions       │
│               └────────┬────────┘                        │
│                        │ Relay(toID, frame)              │
│                        │  • QUIC: SendDatagram           │
│                        │  • TCP:  Stream.Write           │
└────────────────────────┼─────────────────────────────────┘
                         │
         ┌───────────────┴───────────────┐
         ▼                               ▼
  ┌─────────────┐                 ┌─────────────┐
  │  Agent A    │                 │  Agent B    │
  │  QUIC client│                 │  QUIC client│
  └─────────────┘                 └─────────────┘
```

## Non-Goals

- **P2P QUIC between peers** — QUIC cannot replace ICE for NAT traversal. ICE handles hole-punching; QUIC cannot.
- **TLS PKI** — Relay server uses a self-signed certificate at startup. Clients use `InsecureSkipVerify`. Underlying WireGuard encryption provides end-to-end security.
