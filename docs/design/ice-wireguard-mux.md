---
title: ICE + WireGuard Shared Port
---

# ICE + WireGuard Shared Port

> Source: `superpowers/specs/ice-wireguard-mux.md`

## Problem

Lattice uses a single UDP port `:51820` for both ICE signaling (STUN) and WireGuard data traffic. Two goroutines compete for packets on the same `net.UDPConn`:

| Packet type | Taken by `connWorker` | Taken by `makeReceiveIPv4` |
|-------------|----------------------|---------------------------|
| STUN | ✅ correct → ICE | ❌ WireGuard decrypt fails, dropped |
| WireGuard encrypted | ❌ no ufrag match, dropped | ✅ correct |

## Solution: FilteringUDPMux

`FilteringUDPMux` is a custom `UDPMux` that inspects each incoming packet before dispatching:

1. **STUN packet** (starts with `0x00` or `0x01`, magic cookie `0x2112A442`) → dispatch to ICE agent
2. **Non-STUN packet** → dispatch to WireGuard `DefaultBind`

This eliminates the race condition. The ICE agent and WireGuard device each see only their own traffic.

## ICE Agent Lifecycle

After `agent.Dial()` / `Accept()` succeeds:
- ICE agent is closed — `connWorker` goroutine stops
- WireGuard's `PersistentKeepalive` maintains the NAT mapping
- STUN keepalive is not needed and is eliminated

## Implementation

```
internal/agent/transport/
└── filtering_mux.go   # FilteringUDPMux implementation
```

Key function: `isSTUN(buf []byte) bool` — checks the STUN magic cookie at bytes 4-7.
