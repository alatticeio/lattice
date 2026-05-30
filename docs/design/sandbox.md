---
title: Sandbox Architecture
---

# Sandbox Architecture

> Source: `cmd/lattice/cmd/sandbox/`, `internal/agent/gvisor/`, `internal/agent/runsc/`

## Overview

`lattice sandbox start` supports two isolation modes:

| Mode | Flag | Isolation mechanism | Network stack |
|------|------|-------------------|---------------|
| **pod** | `--mode pod` | gVisor user-space netstack (in-process) | gVisor `pkg/tcpip` + TUNAdapter |
| **gvisor** | `--mode gvisor` | gVisor runsc container (`--network=host`) | Pod kernel WireGuard (real `/dev/net/tun`) |

Both modes give AI agents a full Lattice network identity — NATS registration, ICE hole-punching, LRP relay fallback — identical to a regular node.

## gVisor Mode (runsc): Two-Phase Architecture

gVisor cannot simultaneously provide K8s network access (`--network=host`) and a virtual TUN device (`--network=sandbox`). The solution splits work into two phases:

```
Phase 1 (pod kernel):                    Phase 2 (runsc container):
┌──────────────────────────┐            ┌─────────────────────────────┐
│  bootstrapAgent()        │            │  runsc --network=host        │
│                          │            │                             │
│  ① NATS registration    │            │  PID 1: AI agent binary      │
│  ② wireguard-go → wg0   │            │  (direct exec, no shim)      │
│     (real /dev/net/tun)  │            │                             │
│  ③ Routes + iptables    │            │  AI agent connect(peer)      │
│                          │            │    → gVisor sentry           │
│  node stays alive ───────┼────────────▶   → host kernel passthrough │
│                          │            │    → pod routing             │
└──────────────────────────┘            │    → wg0 → overlay           │
                                        └─────────────────────────────┘
```

**Key property**: WireGuard runs on the real kernel, not inside gVisor. The AI agent in gVisor inherits the pod's network namespace via `--network=host`, so its traffic follows the pod's routes into wg0 and the overlay. gVisor's sentry intercepts all syscalls for security isolation, but networking no longer depends on gVisor's internal netstack.

### AI agent traffic path (gVisor mode)

```
AI agent connect(peer-ip:port)
  → gVisor sentry (syscall interposition, security policy)
  → host kernel passthrough (--network=host)
  → pod routing table
  → wg0 (real WireGuard, pod kernel)
  → UDP :51820 → WireGuard peer → overlay
```

### Security

| Layer | Mechanism |
|-------|-----------|
| Syscall isolation | gVisor sentry (all syscalls intercepted) |
| Network access | Pod iptables/eBPF rules on wg0 |
| WireGuard keys | On pod kernel, not in gVisor |
| CAP_NET_ADMIN | Not granted to gVisor container |

## pod Mode (in-process gVisor netstack)

The original architecture uses an in-process gVisor netstack. WireGuard runs in user-space with a TUN adapter bridging gVisor's network stack to the WireGuard device.

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

## Comparison

| Dimension | Regular Node | pod mode | gvisor mode |
|-----------|-------------|----------|-------------|
| Isolation | None | gVisor netstack (in-process) | runsc container |
| Privilege | root / `CAP_NET_ADMIN` | **Zero-privilege** | **Zero-privilege** |
| Network stack | Kernel TUN (`wf0`) | gVisor `pkg/tcpip` + TUNAdapter | Pod kernel TUN (real wg0) |
| WireGuard | Kernel `wgctrl` | `wireguard-go` (user-space) | `wireguard-go` (pod kernel) |
| Provisioner | `KernelProvisioner` (iptables/eBPF) | `SandboxProvisioner` (no iptables) | `KernelProvisioner` (pod iptables/eBPF) |
| Registration | HTTP or NATS | NATS only | NATS only |
| Credential persistence | None | JSON file | JSON file |
| SOCKS5 proxy | None | Optional | None (direct routing) |
| Inbound forwarding | None | Optional (Pro) | None |
| Egress policy | eBPF TC (Pro) / iptables | `EgressFilter` (Pro) | Pod iptables/eBPF |
| ICE / LRP | Full support | Full support | Full support |

## Code Structure

```
cmd/lattice/cmd/sandbox/
├── sandbox.go              # Command definition (--name, --server-url, --token, --mode)
├── sandbox_shared.go       # No build tag — shared utilities (credential I/O, fileAuditWriter)
├── sandbox_community.go    # //go:build !pro — full community implementation (pod mode)
├── sandbox_pro.go          # //go:build pro  — Pro-only extensions (both modes)
├── sandbox_run_pro.go      # //go:build pro  — `lattice sandbox run` (pod & gvisor modes)
├── sandbox_agent.go        # //go:build pro  — `lattice sandbox agent` (manual debugging)
├── driver.go               # DriverConfig, IsolationDriver interface
├── driver_pod.go           # //go:build pro  — PodDriver (in-process gVisor netstack)
└── driver_runsc.go         # //go:build pro  — RunscDriver (two-phase bootstrap + runsc)

internal/agent/
├── gvisor/                 # In-process gVisor netstack (pod mode)
│   ├── sandbox.go          # gvisor.New() entry point
│   ├── tun_adapter.go      # TUNAdapter: gVisor ↔ wireguard-go packet bridge
│   └── provisioner.go      # SandboxProvisioner (no iptables, replaces KernelProvisioner)
└── runsc/                  # runsc OCI container lifecycle (gvisor mode)
    └── runsc.go            # Manager: OCI spec generation, runsc start/stop
```

## Community vs Pro Build Tags

```go
// sandbox_community.go
//go:build !pro

// sandbox_pro.go, sandbox_agent.go, driver_pod.go, driver_runsc.go
//go:build pro
```

Community stubs for Pro features return `"... is a Pro feature"` errors. Build with `make EDITION=pro build` to include Pro code.
