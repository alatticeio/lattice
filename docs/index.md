---
title: Lattice Documentation
---

# Lattice Documentation

Lattice is a **self-hosted WireGuard mesh orchestration platform**. Deploy the control plane on your infrastructure and manage your encrypted overlay network through a web dashboard.

## Quick Links

- [Quick Start](guide/quickstart) — Get a mesh network running in 10 minutes
- [Installation](guide/installation) — Install the CLI and server components
- [All-in-One Deployment](deploy/all-in-one) — Deploy with Docker or K8s
- [Configuration Reference](config/reference) — All config options

## Key Features

- **Self-hosted Dashboard** — Full web UI for managing peers, policies, and monitoring
- **K8s-native + Device-native** — Works for K8s clusters (CRD operator) and personal devices
- **WireGuard tunnels** — Automatic NAT traversal via ICE/STUN/TURN
- **Built-in relay** — LRP with QUIC fallback when direct P2P isn't possible
- **Network policies** — Default-deny, label-based ACLs with eBPF (Pro) or iptables
- **Multi-tenant** — Workspace isolation with RBAC
