# How Lattice Compares

## Overview

| Feature | Lattice | Tailscale | NetBird | Nebula |
|---------|---------|-----------|---------|--------|
| **Self-hosted** | ✅ Any infra | ❌ Cloud-dependent* | ✅ | ✅ |
| **Open-core** | ✅ Apache 2.0 | ❌ Proprietary | ✅ BSD | ✅ MIT |
| **WireGuard mesh** | ✅ | ✅ | ✅ | ❌ (custom proto) |
| **NAT traversal** | ICE + relay | DERP | STUN/TURN | UDP punching |
| **K8s native** | CRD operator | Operator | — | — |
| **Policy engine** | ACL + eBPF (Pro) | ACLs | ACLs | Firewall groups |
| **AI operations** | MCP + intent engine | — | — | — |
| **Compliance reports** | ✅ (Pro) | — | — | — |
| **Multi-tenant** | Workspaces + RBAC | Tailnet ACLs | Groups | — |
| **Relay protocol** | LRP over QUIC | DERP over HTTPS | TURN | Lighthouse |
| **Dashboard UI** | ✅ | ✅ Web | ✅ Web | ❌ CLI only |
| **Audit logging** | ✅ (basic: CE, full: Pro) | ✅ | ✅ | — |
| **Device support** | Linux, macOS, K8s | Most platforms | Most platforms | Most platforms |
| **Community size** | Growing | Large | Medium | Small |

\* Tailscale offers self-hosted "Headscale" but it's a separate community project, not officially supported.

## When to Choose Lattice

- You want **full control** — deploy the control plane on your own infrastructure
- You need **Kubernetes-native** networking — CRD operator, pod-level policies
- You want **AI integration** — MCP Server, natural language management, compliance automation
- You're **budget-conscious** — Community edition is free, Pro adds enterprise features
- You need **multi-tenant isolation** — Workspaces with independent RBAC

## When to Choose Alternatives

- **Tailscale** if you want managed infrastructure (no ops) and broadest device support
- **NetBird** if you want open-source with a polished managed cloud option
- **Nebula** if you're a Slack engineer and want a battle-tested custom protocol
