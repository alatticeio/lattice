# Changelog

All notable user-facing changes are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

Versions follow `MAJOR.MINOR.PATCH`. During the 0.x phase (Public Beta), minor versions may include breaking changes.

---

## Unreleased (v0.2.0)

### Agent Sandbox
- Agent detail drawer in dashboard — trace viewing, network audit log, sub-agent delegation UI
- Token-based enrollment system for sandbox agents

### MCP Server
- External MCP server management page (register, list, delete) in dashboard

### E2E Testing
- Sandbox E2E test infrastructure (Ginkgo-based, covers gVisor startup, WireGuard connectivity, egress filter, forward listener)

### Security
- SaaS security compliance hardening for public deployment

---

## v0.1.0 (2026-05)

First public beta release.

### Highlights

- **AI Agent Platform**: MCP Server (14 tools), AgentIdentity CRD + lifecycle, gVisor zero-privilege sandbox, intent engine (Pro), time-travel debugging (Pro), compliance automation (Pro)
- **Agent Sandbox**: `lattice sandbox start` — gVisor user-space netstack + WireGuard, tool call tracing (`la_tool_spans`), sub-agent delegation API
- **Egress Control (Pro)**: CIDR whitelist filter, HTTP CONNECT forward proxy, ForwardListener
- **Signaling**: HTTP REST API for CLI management
- **Policy Engine**: Rule engine with eBPF TC support (Pro), comprehensive test coverage
- **Network Peering**: Cluster Peering CRD, Network Peering with status cards
- **Audit & Observability**: MCP tool spans, NATS flow audit (Pro), i18n support
- **Multi-tenancy**: Workspace isolation, RBAC (admin/editor/member/viewer), JWT authentication
- **Core Networking**: WireGuard mesh, ICE NAT traversal (STUN/TURN), LRP relay (TCP + QUIC), K8s CRD Operator (kubebuilder)
