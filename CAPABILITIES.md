# Lattice Capability Matrix

> Single source of truth for what Lattice actually delivers. Every capability is verified against actual code, with implementation status and first-release version.
>
> Last updated: 2026-05-18
> Based on: `cmd/lattice/cmd/sandbox/`, `internal/agent/gvisor/`, `api/v1alpha1/`, `internal/server/controller/`

Legend: ✅ Done (merged to master, usable) / 🔨 In Progress (under development) / 📋 Planned (not yet started)

---

## AI Agent Sandbox

| Capability | Community | Pro | Since | Source |
|------------|-----------|-----|-------|--------|
| gVisor zero-privilege sandbox (`lattice sandbox start`) | ✅ | ✅ | v0.1.0 | `internal/agent/gvisor/` |
| AgentIdentity CRD + lifecycle (Pending/Active/Expired/Revoked) | ✅ | ✅ | v0.1.0 | `api/v1alpha1/agent_identity_types.go` |
| Per-agent WireGuard keypair | ✅ | ✅ | v0.1.0 | `sandbox_shared.go:28` |
| Manager reconciler auto-provisions peer config | ✅ | ✅ | v0.1.0 | `internal/server/controller/agent_identity_controller.go` |
| Credential persistence (restart-safe, mode 0600) | ✅ | ✅ | v0.1.0 | `sandbox_shared.go:42-65` |
| Local audit log (JSONL) | ✅ | ✅ | v0.1.0 | `sandbox_community.go:67` |
| MCP Tool Tracing (`la_tool_spans` table) | ✅ | ✅ | v0.1.0 | `internal/server/models/tool_span.go` |
| Sub-agent delegation API (parentRef + spawnableRoles) | ✅ | ✅ | v0.1.0 | `agent_isolation_router.go:47` |
| TTL auto-expiry (transitions to phase=Expired) | ✅ | ✅ | v0.1.0 | `agent_identity_controller.go:60` |
| CIDR EgressFilter (`--egress-allow`) | — | ✅ | v0.1.0 | `sandbox_pro.go:92-105` |
| ForwardListener (`--forward`) | — | ✅ | v0.1.0 | `sandbox_pro.go:236` |
| HTTP CONNECT forward proxy (`--proxy-addr`) | — | ✅ | v0.1.0 | `sandbox_pro.go:279-353` |
| NATS server-side flow audit (`la_flow_events`) | — | ✅ | v0.1.0 | `audit_consumer_pro.go` |
| Egress default-deny + Drop | — | ✅ | v0.1.0 | `sandbox_pro.go:73` |
| MCP Server (14 tools, read runs directly / write requires approval) | ✅ | ✅ | v0.1.0 | `internal/server/mcp/` |
| AI ChatOps panel (Web UI SSE streaming chat) | ✅ | ✅ | v0.1.0 | `fronted/` |
| TTL expiry CRD auto-GC | 📋 | 📋 | — | Not yet implemented |
| Community egress control (simplified `--egress-allow`) | 📋 | — | — | Not yet implemented |
| Domain-level egress filtering (DNS intercept + dynamic IP mapping) | 📋 | 📋 | — | Not yet implemented |
| L7 filtering (HTTP path/method/header) | 📋 | 📋 | — | Not yet implemented |
| Sandbox→NATS audit push (`natsAuditWriter`) | 📋 | 📋 | — | Planned |
| PID-to-TUN binding (eBPF cgroup/connect4) | 📋 | 📋 | — | Not yet implemented |
| seccomp notify sidecar interception | 📋 | 📋 | — | Not yet implemented |
| Firecracker MicroVM sandbox | 📋 | 📋 | — | Long-term |
| Global topology visualization (D3.js force graph) | 📋 | 📋 | — | Long-term |

---

## Network Orchestration

| Capability | Community | Pro | Since | Source |
|------------|-----------|-----|-------|--------|
| WireGuard encrypted tunnels | ✅ | ✅ | v0.1.0 | `cmd/lattice` |
| ICE NAT traversal (STUN/TURN, pion/ice v4) | ✅ | ✅ | v0.1.0 | `internal/agent/transport/` |
| LRP relay fallback (TCP + QUIC / WRRP) | ✅ | ✅ | v0.1.0 | `internal/agent/transport/` |
| K8s CRD Operator (kubebuilder) | ✅ | ✅ | v0.1.0 | `cmd/manager` |
| Built-in IPAM (auto-assign per Workspace) | ✅ | ✅ | v0.1.0 | `internal/server/` |
| Cluster Peering CRD | ✅ | ✅ | v0.1.0 | `api/v1alpha1/` |
| Network Peering | ✅ | ✅ | v0.1.0 | `features/network-peering` |
| Multi-platform (Linux/macOS/Windows) | ✅ | ✅ | v0.1.0 | — |
| NATS signaling (JetStream bidirectional) | ✅ | ✅ | v0.1.0 | — |
| Dashboard UI | ✅ | ✅ | v0.1.0 | `fronted/` |
| Multi-Workspace isolation | ✅ | ✅ | v0.1.0 | config/kustomize |
| RBAC (admin/editor/member/viewer) | ✅ | ✅ | v0.1.0 | `internal/server/` |
| JWT authentication (user JWT + agent JWT + revocation list) | ✅ | ✅ | v0.1.0 | `internal/server/` |
| CLI management via HTTP REST (replaces NATS) | ✅ | ✅ | v0.1.0 | — |

---

## Policy Enforcement

| Capability | Community | Pro | Since | Source |
|------------|-----------|-----|-------|--------|
| LatticePolicy CRD (Ingress/Egress + label selector) | ✅ | ✅ | v0.1.0 | `api/v1alpha1/lattice_policy_types.go` |
| Default-Deny semantics | ✅ | ✅ | v0.1.0 | — |
| Policy TTL auto-expiry | ✅ | ✅ | v0.1.0 | `LatticePolicy.ExpiresAt` |
| iptables policy enforcement | ✅ | — | v0.1.0 | Community enforcer |
| eBPF TC ingress enforcement (kernel-level) | — | ✅ | v0.1.0 | `internal/agent/ebpf/` |
| pfctl policy enforcement (macOS) | ✅ | — | v0.1.0 | — |

---

## Current Gaps & Next Steps

1. **Domain/L7 filtering** — DNS intercept + dynamic IP mapping in gVisor netstack, medium effort
2. **CRD GC** — Delete AgentIdentity / LatticePeer on TTL expiry, small effort
3. **natsAuditWriter** — Push sandbox audit events to NATS, planned
4. **Community egress control** — At minimum, a simplified `--egress-allow` for Community edition
5. **PID-to-TUN binding** — eBPF `cgroup/connect4` to force agent traffic through WireGuard, blocking direct eth0 bypass

---

## Competitive Differentiation

| Dimension | Calico / Cilium | Tailscale / Netbird | Lattice |
|-----------|----------------|---------------------|---------|
| Core focus | K8s CNI plugin | Enterprise VPN mesh | AI Agent zero-trust sandbox + overlay network |
| Agent identity | Pod IP / ServiceAccount | Device/User | WireGuard public key + AgentIdentity CRD |
| Agent isolation | NetworkPolicy (IP/port) | ACLs | gVisor user-space sandbox + WireGuard identity |
| Zero-privilege deployment | Requires privileged DaemonSet | Requires root/TUN | `lattice sandbox start` needs no root |
| AI framework integration | None | None | MCP Server + Python SDK |
| TTL auto-expiry | None | None | AgentIdentity.ExpiresAt + LatticePolicy.ExpiresAt |
| Audit trail | External systems | External systems | MCP tool spans + NATS flow audit (Pro) |
| Cross-cluster/region | Requires multi-cluster mesh | Native (overlay) | Native Cluster Peering CRD |
