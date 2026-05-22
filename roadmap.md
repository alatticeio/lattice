# Roadmap

> Last updated: 2026-05-18
>
> Positioning: Lattice = AI Agent security infrastructure (identity + network + isolation)
>
> Current capabilities: [CAPABILITIES.md](./CAPABILITIES.md)
> Long-term vision: [docs/superpowers/specs/2026-05-16-lattice-future-vision-and-roadmap.md](docs/superpowers/specs/2026-05-16-lattice-future-vision-and-roadmap.md)

---

## Now — v0.2 Current Iteration

Goal: **Close the "read-and-burn" loop + lower adoption friction**

| # | Task | Effort | Description |
|---|------|--------|-------------|
| 1 | TTL expiry CRD auto-GC | S | Auto-delete AgentIdentity/LatticePolicy resources on expiry |
| 2 | Community egress control | M | Basic `--egress-allow` CIDR whitelist for Community edition |
| 3 | natsAuditWriter implementation | M | Push sandbox audit events to NATS → latticed storage |
| 4 | Capability matrix public docs | S | Sync CAPABILITIES.md to docs site |

---

## Next — v0.3 Deep Isolation

Goal: **From network isolation to process isolation — agents cannot bypass the sandbox**

| # | Task | Effort | Description |
|---|------|--------|-------------|
| 5 | Domain-level egress filtering | M | DNS intercept + dynamic IP mapping + TTL caching inside gVisor netstack |
| 6 | PID-to-TUN binding (eBPF cgroup/connect4) | L | Force agent process traffic through WireGuard, block direct eth0 bypass |
| 7 | seccomp notify sidecar | L | Zero-instrumentation agent egress interception via seccomp user-space notify → sandbox policy decision |
| 8 | SandboxPod mode | M | `sandbox: pod` — K8s Pod + seccomp lightweight isolation (Community) |

---

## Later — v0.4+ Platform

| # | Task | Effort | Description |
|---|------|--------|-------------|
| 9 | L7 HTTP filtering (path/method/header) | M | HTTP parsing + rule matching inside gVisor netstack |
| 10 | Global topology visualization | M | D3.js force graph, real-time P2P/LRP path display |
| 11 | Firecracker MicroVM sandbox (Pro) | L | `sandbox: microvm` — true hardware-level isolation |
| 12 | Managed control plane MVP (SaaS) | L | Optional lightweight hosted control plane, policy declarations only (no traffic) |
| 13 | Helm chart | S | Standard Helm deployment |
| 14 | Prometheus + Grafana integration | M | Standard monitoring integration |

---

## Priority Principles

1. **Security closure over polish** — Incomplete network isolation (eth0 bypass) is the biggest security gap; fix it first
2. **Community covers core scenarios** — Essential capabilities should not hide behind the Pro tier
3. **Every release eliminates entries from CAPABILITIES.md's 📋 column**
