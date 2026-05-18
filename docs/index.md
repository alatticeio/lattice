---
layout: home

hero:
  name: "Lattice"
  text: "WireGuard Overlay Network for AI Workloads"
  tagline: Zero-privilege agent sandbox · Kubernetes-native · Open-core
  image:
    src: /logo.svg
    alt: Lattice
  actions:
    - theme: brand
      text: Quick Start →
      link: /guide/quickstart
    - theme: alt
      text: Agent Platform
      link: /agent/
    - theme: alt
      text: GitHub
      link: https://github.com/alatticeio/lattice

features:
  - icon: 🤖
    title: Agent Sandbox
    details: gVisor zero-privilege sandbox for AI agents — NATS registration, ICE/LRP tunneling, credential persistence. Community Edition.
  - icon: 🔐
    title: WireGuard Mesh
    details: Automatic NAT traversal via ICE/STUN/TURN with built-in LRP relay over QUIC
  - icon: ☸️
    title: K8s Native
    details: CRD operator (kubebuilder) for Kubernetes clusters, CLI agent for devices — same network plane
  - icon: 🛡️
    title: Network Policies
    details: Default-deny, label-based ACLs with eBPF (Pro) or iptables enforcement
  - icon: 🧠
    title: MCP + ChatOps
    details: Natural language network management via MCP Server — works with Claude, Cursor, and any MCP client
  - icon: 🏷️
    title: Open-Core
    details: Community edition is free forever. Pro adds eBPF, NATS audit, compliance automation, and AI debugging
---

## Feature Map

| Module | Community | Pro |
|--------|-----------|-----|
| WireGuard Tunnels | ✅ | ✅ |
| NAT Traversal (ICE/STUN/TURN) | ✅ | ✅ |
| LRP Relay (QUIC) | ✅ | ✅ |
| K8s CRD Operator | ✅ | ✅ |
| Dashboard UI | ✅ | ✅ |
| Agent Sandbox (gVisor) | ✅ | ✅ + EgressFilter |
| Sub-agent Delegate API | ✅ | ✅ |
| MCP Tool Tracing | ✅ | ✅ |
| Label-based ACLs | ✅ (iptables) | ✅ (eBPF) |
| Cluster Peering | ✅ | ✅ |
| Multi-Tenant Workspaces | ✅ | ✅ |
| MCP Server & ChatOps | ✅ | ✅ |
| Network Topology Map | ✅ | ✅ |
| NATS Flow Audit | — | ✅ |
| Policy Engine | Basic | Advanced |
| Time-Travel Debugging | — | ✅ |
| Compliance Reports | — | ✅ |
| Audit Logging | Basic | Advanced |

## Ready to Try?

[Quick Start](/guide/quickstart) · [Agent Platform](/agent/) · [GitHub](https://github.com/alatticeio/lattice)
