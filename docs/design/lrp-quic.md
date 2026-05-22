---
title: LRP / QUIC Relay Transport
---

# LRP / QUIC Relay Transport

> Source: `superpowers/specs/design-wrrp-quic.md`
>
> **命名说明**: 代码中已从 WRRP (Wireflow Relay & Routing Protocol) 重命名为 LRP (Lattice Relay Protocol)。本文档保留原始设计内容，仅修正标题。代码文件使用 `lrp_` 前缀（如 `lrp_server.go`、`lrp_client_quic.go`）。

## Background

LRP (Lattice Relay Protocol, formerly WRRP) is the relay channel used when two peers cannot establish a direct ICE path. The original implementation tunnels WireGuard packets over a persistent HTTP-upgraded TCP connection.

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
│                    LRP Relay Server                     │
│                                                          │
│  ┌─────────────────────┐   ┌─────────────────────────┐  │
│  │   TCP Server        │   │   QUIC Server           │  │
│  │   :6266 (HTTP upg.) │   │   :6267 (UDP)           │  │
│  └────────┬────────────┘   └──────────┬──────────────┘  │
│           └──────────┐   ┌────────────┘                  │
│                      ▼   ▼                               │
│               ┌─────────────────┐                        │
│               │  LRPManager    │                        │
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
