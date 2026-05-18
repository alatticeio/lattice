---
title: Architecture Overview
---

# Architecture Overview

Lattice is composed of three core components:

| Component | Entry Point | Purpose |
|-----------|-------------|---------|
| **Lattice Agent** | `cmd/lattice` | Edge node: WireGuard tunnel, NATS signaling, ICE/LRP |
| **LatticeD** | `cmd/latticed` | All-in-one control plane: NATS + SQLite + API + UI |
| **Manager** | `cmd/manager` | Kubernetes operator: reconciles CRDs |

## Transport Layer

Each peer-to-peer connection goes through a state machine:

```
Created → Probing → ICEReady / LRPReady → Failed → Closed
```

ICE (direct P2P) and LRP (relay fallback) race in parallel. Whichever establishes first wins. The connection upgrades from LRP to ICE transparently when a direct path becomes available.

```
Peer A                                    Peer B
  │  ── NATS signal ──────────────────────▶  │
  │  ◀── NATS signal ──────────────────────  │
  │                                          │
  │  ── ICE probe ─────────────────────────▶ │  ← races with LRP
  │  ── LRP connect ───────────▶ Relay ────▶ │
  │                                          │
  └─────────── WireGuard data ───────────────┘
```

## Policy Enforcement

`PolicyEnforcer` interface in `internal/agent/provision/provisioner.go`:

```
SelectEnforcerMode()
  ├── Linux 5.10+ && EDITION=pro → eBPF TC on wf0 TUN interface
  └── default                   → iptables
```

Community edition always uses iptables. Pro edition uses eBPF when kernel supports it, with automatic fallback to iptables.

## Agent Sandbox

The sandbox provides a **zero-privilege** alternative to the regular node:

- Regular node: kernel TUN (`wf0`), requires `CAP_NET_ADMIN`
- Sandbox: gVisor `pkg/tcpip` + `TUNAdapter`, runs as any user

See [Sandbox Architecture](/design/sandbox) for the detailed design.

## Signal Flow

```
LatticeD (NATS broker)
    │
    ├── lattice.signals.peers.<PeerID>  ← peer signaling
    ├── lattice.audit.flow              ← Pro: NATS flow audit
    └── lattice.config.<PeerID>         ← peer config push
```
