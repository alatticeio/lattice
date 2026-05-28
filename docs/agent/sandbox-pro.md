---
title: Agent Sandbox (Pro)
---

# Agent Sandbox — Pro Edition

The Pro sandbox extends the [Community sandbox](/agent/sandbox) with egress filtering, inbound port forwarding, a SOCKS5 proxy, and server-side NATS flow auditing.

## `sandbox run` — One-Command Sandbox

`lattice sandbox run` combines `start` + agent execution + cleanup into a single command. It supports two isolation modes:

```bash
# pod mode (default) — SOCKS5 proxy + ALL_PROXY injection
lattice sandbox run --name my-agent --server-url http://latticed:8080 --token lt-xxx \
  -- python agent.py

# gvisor mode — runsc container isolation
lattice sandbox run --name my-agent --server-url http://latticed:8080 --token lt-xxx \
  --mode gvisor --agent-rootfs /var/lib/lattice/rootfs \
  -- python agent.py
```

**pod mode** starts a network sandbox, injects `ALL_PROXY` into the child process, and executes the given command. When the child exits, the sandbox is automatically cleaned up.

**gvisor mode** runs the agent binary directly inside a gVisor runsc container. The first arg after `--` becomes the agent binary (unless `--agent-binary` is explicitly set).

| Flag | Mode | Description |
|------|------|-------------|
| `--mode` | both | Isolation mode: `pod` (default) or `gvisor` |
| `--agent-rootfs` | gvisor | Root filesystem path for runsc container |
| `--agent-binary` | gvisor | Agent entrypoint binary (defaults to first arg after `--`) |
| `--agent-args` | gvisor | Arguments passed to the agent binary |
| `--proxy-addr` | pod | SOCKS5 proxy listen address (empty = random port) |
| `--ready-timeout` | pod | Max wait for sandbox ready (default 30s) |
| `--wg-port` | pod | WireGuard UDP port (0 = random) |

## Additional Flags (`sandbox start`)

```
lattice sandbox start [flags]

Pro-only flags (pod mode):
  --egress-allow   strings   CIDR ranges to allow outbound (default: deny-all)
  --forward        strings   Overlay port → host address mappings (e.g. 8080:127.0.0.1:8080)
  --proxy-addr     string    Local address for SOCKS5 proxy (e.g. 127.0.0.1:1080)

Pro-only flags (gvisor mode):
  --mode gvisor              Enable gVisor runsc isolation
  --agent-rootfs  string     Path to container root filesystem (required for gvisor mode)
  --agent-binary  string     AI agent entrypoint binary inside container (required)
  --agent-args    strings    Arguments passed to the AI agent binary
```

## EgressFilter

EgressFilter implements `PolicyChecker` — it intercepts every outbound connection attempt from the gVisor netstack and evaluates it against a CIDR allowlist. In **pod mode**, filtering happens inside gVisor's user-space netstack. In **gVisor mode**, egress filtering is enforced by pod iptables/eBPF on the real wg0 interface.

```bash
# Only allow outbound to the tool service at 10.42.0.10 and HTTPS
lattice sandbox start \
  --name agent-001 \
  --server-url http://lattice.internal:8080 \
  --token lt-xxx \
  --egress-allow 10.42.0.10/32 \
  --egress-allow 0.0.0.0/0:443
```

If no `--egress-allow` flags are provided, all egress is denied. Denied connections are logged to the audit file.

## ForwardListener (pod mode only)

Forward inbound connections from the overlay network to a host-local address.
Not applicable in gVisor mode.

```bash
# Accept connections on overlay port 8080, forward to localhost:8080
lattice sandbox start \
  --name api-agent \
  --server-url http://lattice.internal:8080 \
  --token lt-xxx \
  --forward 8080:127.0.0.1:8080
```

Multiple `--forward` flags are supported.

## SOCKS5 Proxy (pod mode only)

Start a SOCKS5 proxy inside the gVisor sandbox (pod mode). All TCP connections
through this proxy are routed through the WireGuard overlay with full policy
enforcement and audit logging. Not applicable in gVisor mode — the AI agent
connects to overlay IPs directly.

```bash
lattice sandbox start \
  --name browser-agent \
  --server-url http://lattice.internal:8080 \
  --token lt-xxx \
  --proxy-addr 127.0.0.1:1080
```

In your agent process:

```bash
export ALL_PROXY=socks5://127.0.0.1:1080
curl https://internal-api.example.com/data
```

## NATS Flow Audit (Server-side)

Pro adds server-side persistence of network flow events. The sandbox publishes to `lattice.audit.flow` on NATS; the control plane's `AuditConsumer` persists events to the `la_flow_events` database table.

> **Status:** Server-side pipeline is complete (`AuditConsumer` + `la_flow_events`). The sandbox-side `natsAuditWriter` is in development — sandbox currently writes to local file only.

Query audit events via the dashboard or API:

```
GET /api/v1/audit/flow?agentName=agent-001&from=2026-05-18T00:00:00Z
```

## gVisor runsc Mode

`--mode gvisor` provides syscall-level isolation by running the AI agent inside a gVisor runsc container. This mode uses a **two-phase architecture**:

```
Phase 1 (pod kernel):                   Phase 2 (runsc --network=host):
  bootstrapAgent()                       PID 1: AI agent binary
  ① NATS registration                   gVisor sentry intercepts all
  ② wireguard-go → wg0 (real TUN)       syscalls, but networking is
  ③ Routes + iptables on pod            handled by the pod kernel wg0
```

The AI agent traffic flows: `gVisor sentry → host kernel passthrough → pod routing → wg0 → WireGuard → overlay`. gVisor provides security isolation (syscall interception), while all networking runs on the real kernel.

### gVisor Mode Flags

```bash
lattice sandbox start \
  --mode gvisor \
  --name agent-001 \
  --server-url http://latticed:8080 \
  --token lt-xxx \
  --agent-rootfs /opt/lattice/agent-rootfs \
  --agent-binary /usr/local/bin/ai-agent \
  --agent-args --model,gpt-4
```

| Flag | Required | Description |
|------|----------|-------------|
| `--mode gvisor` | Yes | Enable gVisor runsc isolation |
| `--agent-rootfs` | Yes | Path to container root filesystem |
| `--agent-binary` | Yes | AI agent binary inside the container |
| `--agent-args` | No | Arguments passed to the AI agent binary |
| `--egress-allow` | No | CIDR ranges to allow outbound (enforced by pod iptables/eBPF) |
| `--egress-default-deny` | No | Whitelist egress mode |

### Security Boundary

| Layer | Mechanism |
|-------|-----------|
| Syscall isolation | gVisor sentry intercepts all AI agent syscalls |
| Network access | Pod iptables/eBPF on wg0 |
| WireGuard keys | On pod kernel, never inside gVisor |
| CAP_NET_ADMIN | Not granted to gVisor container |
| TUN device | Not available inside gVisor |
