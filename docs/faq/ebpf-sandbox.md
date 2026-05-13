# Frequently Asked Questions

Common questions and answers about Lattice internals, architecture, and design decisions.

### eBPF & Policy Enforcement
- [How does eBPF coexist with iptables?](#ebpf-vs-iptables)
- [Why did the PRO build fail with "undefined: ebpf.NewManager"?](#pro-build-fix)
- [What is the PRO/Community build tag pattern?](#build-tags)

### Agent Sandbox
- [How does the Agent sandbox work?](#sandbox-overview)
- [What's the difference between Phase 1 and Phase 2 sandbox?](#phase1-vs-phase2)
- [Is there a kernel-to-userspace boundary in the gVisor sandbox?](#kernel-userspace-boundary)
- [Should infrastructure nodes use gVisor?](#infra-vs-gvisor)

---

## eBPF & Policy Enforcement

### eBPF vs iptables {#ebpf-vs-iptables}

**Does eBPF replace all iptables rules in Lattice?**

No. eBPF only replaces the **policy enforcement** layer (allow/drop decisions on wf0). iptables is still used for:

| Responsibility | Implementation | Can eBPF replace it? |
|---------------|----------------|---------------------|
| Policy enforcement (allow/drop) | eBPF `tc_ingress` (PRO) / iptables (community) | ✅ Yes |
| Docker NAT (MASQUERADE) | iptables | ❌ No |
| FORWARD rules (wf0↔eth0) | iptables | ❌ No |
| Community fallback | iptables | ❌ No (kept intentionally) |

```
Policy layer:  eBPF TC ingress (PRO) or iptables (community)
    ↓
Forwarding:    iptables FORWARD  ← needed for both PRO and community
    ↓
NAT:           iptables MASQUERADE  ← needed for both, eBPF can't do this
```

### PRO Build Fix {#pro-build-fix}

**Why did `make docker-build SERVICE=lattice EDITION=pro` fail with `undefined: ebpf.NewManager` and `undefined: ebpf.HaveProgramType`?**

Two issues introduced in commit `01f73381` (ebpf PR #221):

1. **`ebpf.NewManager` undefined**: `manager_community.go` has `//go:build !pro` and provides a stub for community builds. When building with `-tags pro`, this file is excluded, but no `manager_pro.go` was created to provide the real implementation.

2. **`ebpf.HaveProgramType` undefined**: In cilium/ebpf v0.21.0, `HaveProgramType` moved from the root package to `github.com/cilium/ebpf/features`, and its signature changed from `(bool, error)` to `(error)` (nil = supported).

**Fix:**

- Updated `enforcer_selector_pro.go` to use `features.HaveProgramType(ebpf.SchedCLS)` and check `err != nil`.
- Created `internal/agent/ebpf/manager_pro.go` (`//go:build pro`) with the real eBPF Manager: loads TC ingress program, attaches via `link.AttachTCX`, and manages BPF maps.

These issues weren't caught earlier because the PRO build path wasn't triggered in CI previously.

### Build Tags {#build-tags}

**How does the PRO/Community build tag pattern work?**

```
//go:build pro      → compiled only with -tags pro (real implementation)
//go:build !pro     → community stub, returns 402 / "Pro feature" error
No build tag       → always compiled
```

Makefile: `EDITION ?= community`. When `EDITION=pro`, `BUILD_TAGS := -tags pro` is set.

---

## Agent Sandbox

### Sandbox Overview {#sandbox-overview}

**What is the Lattice Agent sandbox?**

The Agent sandbox ensures AI Agent network traffic is controlled by Lattice policy. It provides process-level isolation so that agents can only communicate with authorized destinations.

Two parallel approaches serve the same goal:

| | Phase 1 (Kernel-assisted) | Phase 2 (User-space kernel) |
|---|---|---|
| Interception point | Kernel eBPF TC hook on wf0 | User-space Sentry netstack |
| Mechanism | cgroup + eBPF + seccomp notify | gVisor (user-space TCP/IP stack) |
| Privilege needed | `CAP_NET_ADMIN` + `CAP_BPF` | **Zero** |
| Performance | High (kernel-space) | Moderate (sufficient for agents) |
| Best for | Infrastructure nodes (DB, K8s mesh) | AI Agent sandboxes |

### Phase 1 vs Phase 2 {#phase1-vs-phase2}

**Are Phase 1 and Phase 2 progressive stages or alternative approaches?**

They are **parallel implementations**, not progressive stages. Both achieve the same goal (policy-controlled agent networking) but differ in where policy enforcement happens:

- **Phase 1**: Policy enforced in the kernel (eBPF TC hook on wf0 TUN device). Higher performance, requires root.
- **Phase 2**: Policy enforced in user-space (gVisor Sentry Go netstack). Zero privilege, moderate performance.

PRO users can choose per workload: infrastructure nodes use Phase 1 eBPF, AI agent sandboxes use Phase 2 gVisor.

**What is Phase 1's current implementation status?**

Phase 1 is **not yet implemented**. The design document exists, but `internal/agent/sandbox/`, `internal/agent/sidecar/`, and `internal/agent/ebpf/sandbox_bind.bpf.c` have not been created. The design also needs revision: it labels Phase 1 as "community edition" but heavily uses eBPF, which is a PRO feature.

**Revised layering:**

```
Community Phase 1:
  cgroup resource limits (cpu/mem/io)
  + LD_PRELOAD socket interception (liblattice_intercept.so, zero privilege)
  + iptables policy enforcement

PRO Phase 1 enhancement:
  All of community
  + eBPF PID→TUN binding (cgroup/connect4, zero seccomp overhead)
  + eBPF fast path (whitelist hit bypasses seccomp)
  + eBPF TC ingress policy (replaces iptables)

PRO Phase 2:
  gVisor full user-space isolation (zero privilege, ✅ implemented)
```

### Kernel-to-Userspace Boundary {#kernel-userspace-boundary}

**Is there a kernel→userspace transition in the gVisor sandbox?**

No. This is the key architectural difference:

**Phase 1 (community, with eBPF)** — there IS a kernel boundary:
```
Agent process connect("api.internal", 443)
  → enters kernel socket layer      ← kernel space
  → eBPF cgroup/connect4 triggers    ← THE BOUNDARY!
    checks pid_iface_map: PID 12345 → ifindex=wf0
    forces ctx->user_ifindex = wf0
  → socket hijacked to wf0 TUN      ← enters sandbox network
```

**Phase 2 (gVisor)** — NO kernel boundary for networking:
```
Agent process connect("api.internal", 443)
  → intercepted by Sentry in user-space (never reaches kernel)
  → Go netstack handles everything in user-space
  → wireguard-go encrypts → raw socket sendmsg() → host kernel (L2 only)
```

The raw socket at the end sends already-encrypted WireGuard payloads. The host kernel only does L2 forwarding — it never touches the IP stack. Policy was already enforced at the Sentry layer. eBPF is intentionally absent from the gVisor path.

**Why not add eBPF at the gVisor raw socket boundary?**

1. Policy is already enforced at Sentry layer — adding eBPF would be redundant
2. Zero privilege is gVisor's core value — adding eBPF requires `CAP_BPF`
3. Audit goes through Go channel (`ns.auditCh`), no need for eBPF ring buffer

### Infrastructure vs gVisor {#infra-vs-gvisor}

**Should infrastructure nodes use gVisor?**

No. Infrastructure workloads (database replication, K8s Service mesh, cross-cluster bridging) should use the kernel TUN + eBPF path, not gVisor.

| | Infrastructure | AI Agent Sandbox |
|---|---|---|
| Traffic volume | High-throughput east-west (DB replication, mesh) | Low per-agent traffic |
| Performance requirement | Kernel line-rate | User-space is sufficient |
| Privilege acceptance | Ops nodes, root is fine | Zero-trust, avoid if possible |
| Syscall compatibility | DB/middleware may use obscure syscalls | Agent code is mostly normal network I/O |
| Overhead | eBPF near-zero overhead | gVisor ~25MB/sandbox acceptable |

gVisor's core value is **zero privilege** — giving an untrusted AI Agent process a complete but isolated network stack. Infrastructure nodes are controlled environments: operators already have root, so there's no reason to pay gVisor's performance and compatibility cost. The kernel eBPF path is the right choice for these workloads.

---

## Key Files Reference

| File | Purpose |
|------|---------|
| `internal/agent/ebpf/manager_community.go` | Community stub (`//go:build !pro`) |
| `internal/agent/ebpf/manager_pro.go` | PRO eBPF Manager (`//go:build pro`) |
| `internal/agent/ebpf/tc_ingress.bpf.c` | eBPF C source (LPM Trie + Port Hash policy) |
| `internal/agent/ebpf/tc_ingress_bpfel.go` | bpf2go generated Go bindings |
| `internal/agent/provision/enforcer_selector_pro.go` | PRO eBPF capability detection |
| `internal/agent/provision/enforcer_selector_community.go` | Community: always ModeIPTables |
| `internal/agent/provision/ebpf_enforcer.go` | Creates eBPF enforcer, fallback to iptables |
| `internal/agent/provision/provision_linux.go` | iptables policy + NAT + FORWARD management |
| `internal/agent/gvisor/sandbox.go` | gVisor sandbox core (`//go:build pro`) |
| `internal/agent/gvisor/shim_adapter.go` | PolicyChecker + AuditWriter adapters |
| `docs/superpowers/specs/2026-05-11-agent-sandbox-and-ecosystem-design.md` | Full Phase 1/2/3 design doc |

## Glossary

| Term | Meaning |
|------|---------|
| `wf0` | WireGuard TUN device name, Lattice data-plane entry point |
| TC ingress | Traffic Control ingress hook, eBPF program attach point |
| TCX | cilium/ebpf v0.21.0 TC attach API (replaces old netlink approach) |
| LPM Trie | Longest Prefix Match, eBPF map type for CIDR matching |
| Sentry | gVisor user-space kernel, intercepts sandbox process syscalls |
| PolicyCache | Go-layer policy cache, enforcement point for gVisor path |
| AF_PACKET | Raw socket used by gVisor to send encrypted packets to host kernel |
| LD_PRELOAD | `liblattice_intercept.so` injection, community socket hijack fallback |
| seccomp notify | User-space seccomp notification, Phase 1 agent interception |

---

*Last updated: 2026-05-12*
