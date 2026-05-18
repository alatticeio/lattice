---
title: Agent Platform
---

# Agent Platform

Lattice provides two ways for AI agents to join the WireGuard mesh:

| Method | Command | Isolation | Privilege |
|--------|---------|-----------|-----------|
| **Regular node** | `lattice up` | None (host process) | root / `CAP_NET_ADMIN` |
| **Sandbox** | `lattice sandbox start` | gVisor user-space netstack | **Zero-privilege** |

The Sandbox is the recommended approach for AI agent workloads — it runs entirely in user space, requires no kernel capabilities, and integrates with the same NATS signaling, ICE, and LRP infrastructure as regular nodes.

## Community vs Pro

| Capability | Community | Pro |
|-----------|-----------|-----|
| gVisor user-space network stack | ✅ | ✅ |
| NATS registration + ICE/LRP tunneling | ✅ | ✅ |
| Credential persistence (restart-safe) | ✅ | ✅ |
| Local file audit (`/tmp/lattice-audit-<name>.jsonl`) | ✅ | ✅ |
| Egress policy filtering (`EgressFilter`, `--egress-allow`) | ❌ | ✅ |
| Inbound port forwarding (`--forward`) | ❌ | ✅ |
| SOCKS5 proxy (`--proxy-addr`) | ❌ | ✅ |
| NATS flow audit (server-side `la_flow_events`) | ❌ | ✅ |

## Sub-agent Architecture

Agents can delegate identity to child agents via the **Delegate API** — a parent agent issues a short-TTL token that a sub-agent uses to self-register with a constrained identity (`AgentIdentity.spec.parentRef`).

## Quick Navigation

- [Sandbox (Community)](/agent/sandbox) — full guide: startup flow, CLI reference, credential persistence, AI framework integration
- [Sandbox (Pro)](/agent/sandbox-pro) — EgressFilter, ForwardListener, SOCKS5 proxy, NATS audit
- [Sub-agent Delegate API](/agent/delegate-api) — CRD fields, HTTP endpoint, curl and Python examples
