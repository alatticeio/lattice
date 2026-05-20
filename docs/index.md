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

## 为什么是现在 / Why Now

**谷歌于 2023 年发布 [Secure AI Framework (SAIF)](https://safety.google/cybersecurity-advancements/saif/)**，将 AI 系统的网络隔离、访问管理和供应链安全明确列为企业 AI 部署的核心要求。随着企业开始落地 SAIF，AI agent 的网络边界从"可选项"变为合规要求。

| SAIF 要求 | Lattice 对应能力 |
|---|---|
| 网络 / 端点安全 | WireGuard 加密 mesh，所有 agent 流量端到端加密隔离 |
| 供应链攻击防护 | gVisor 用户态内核，被攻陷的 agent 无法逃逸到宿主机 |
| 访问管理 | Policy 层精确控制每个 agent 身份可访问的资源 |
| 统一平台管控 | 单一 K8s-native 控制面，统一管理 agent、隧道与网络策略 |

**Google published the [Secure AI Framework (SAIF)](https://safety.google/cybersecurity-advancements/saif/) in 2023**, explicitly identifying network isolation, access management, and supply chain security as core requirements for enterprise AI. As organizations implement SAIF, AI agent network boundaries shift from optional to a compliance requirement. Lattice provides the infrastructure layer to meet them — self-hosted, auditable, and open-core.

## Ready to Try?

[Quick Start](/guide/quickstart) · [Agent Platform](/agent/) · [GitHub](https://github.com/alatticeio/lattice)
