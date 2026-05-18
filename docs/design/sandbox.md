---
title: Sandbox Architecture
---

# Sandbox Architecture

> Source: `cmd/lattice/cmd/sandbox/`, `internal/agent/gvisor/`

## Overview

`lattice sandbox start` fuses the **gVisor user-space network stack** with the **Lattice WireGuard overlay**. AI agent processes run as ordinary users with no kernel capabilities, yet obtain a full Lattice network identity — NATS registration, ICE hole-punching, LRP relay fallback — identical to a regular node.

## Comparison with Regular Node

| Dimension | Regular Node (`lattice up`) | Sandbox (`lattice sandbox start`) |
|-----------|---------------------------|----------------------------------|
| Isolation | None (host process) | gVisor user-space netstack |
| Privilege | root / `CAP_NET_ADMIN` | **Zero-privilege** |
| Network stack | Kernel TUN (`wf0`) | gVisor `pkg/tcpip` + TUNAdapter |
| WireGuard | Kernel `wgctrl` | `golang.zx2c4.com/wireguard` (user-space) |
| Provisioner | `KernelProvisioner` (iptables/eBPF) | `SandboxProvisioner` (no iptables) |
| Registration | HTTP or NATS | NATS only (`RegisterSandboxViaNATS`) |
| Credential persistence | None | JSON file (`/etc/lattice/sandbox-credentials.json`) |
| Audit log | eBPF ring buffer (Pro) | JSONL file (`/tmp/lattice-audit-<name>.jsonl`) |
| Egress policy | eBPF TC (Pro) / iptables | `EgressFilter` (Pro sandbox only) |
| Inbound forwarding | None | `ForwardListener` (Pro) |
| HTTP proxy | None | HTTP forward proxy (Pro) |
| ICE / LRP | ✅ Full support | ✅ Full support (shared infrastructure) |

## Network Architecture

```
                ┌─────────────────────────────┐
                │       gVisor Sandbox         │
                │                              │
  Agent process ──▶  gVisor netstack (tcpip)   │
  connect()         │                          │
                │  [Pro] EgressFilter          │
                │   │                          │
                │   TUNAdapter (channel bridge)│
                │   │                          │
                │   wireguard-go Device        │
                └──────────┬───────────────────┘
                           │ UDP :51820
                ┌──────────▼───────────────────┐
                │   FilteringUDPMux             │
                │   STUN ──▶ ICE agent          │
                │   non-STUN ──▶ WG DefaultBind │
                └──────────┬───────────────────┘
                           │
          ┌────────────────┴──────────────┐
          │  ICE succeeds                 │  ICE fails
          ▼                               ▼
    Direct P2P                    LRP relay (QUIC/TCP)
```

The sandbox uses the **same signaling path** as a regular node. gVisor replaces only the kernel TUN device.

## Code Structure

```
cmd/lattice/cmd/sandbox/
├── sandbox.go              # Command definition (--name, --server-url, --token)
├── sandbox_shared.go       # No build tag — shared utilities (credential I/O, fileAuditWriter)
├── sandbox_community.go    # //go:build !pro — full community implementation
└── sandbox_pro.go          # //go:build pro  — Pro-only extensions

internal/agent/gvisor/
├── sandbox.go              # gvisor.New() entry point, Config{ID, LocalIP, PolicyChecker, AuditWriter}
├── tun_adapter.go          # NewTUNAdapter: gVisor ↔ wireguard-go packet bridge
├── provisioner.go          # SandboxProvisioner (no iptables, replaces KernelProvisioner)
└── shimfwd/
    ├── egress_filter.go    # EgressFilter (CIDR allowlist/denylist, implements PolicyChecker)
    ├── forward_listener.go # ForwardListener (overlay port → host address forwarding)
    └── audit_writer.go     # AuditWriter interface + AuditEvent struct
```

## Community vs Pro Build Tags

```go
// sandbox_community.go
//go:build !pro

// sandbox_pro.go
//go:build pro
```

Community stubs for Pro features return `"... is a Pro feature"` errors. Build with `make EDITION=pro build` to include Pro code.
