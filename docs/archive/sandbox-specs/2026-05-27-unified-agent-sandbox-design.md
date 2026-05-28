# Unified Agent Sandbox Design

**Date**: 2026-05-27
**Status**: Approved
**Branch**: dev

## Background

The current sandbox implementation has three divergent paths:

| Mode | Command | Network | Platform |
|------|---------|---------|---------|
| Community sandbox | `sandbox start` | gVisor netstack library (no SOCKS5) | Linux |
| Pod sandbox (PRO) | `sandbox run --mode pod` | gVisor netstack library + SOCKS5 | PRO + Linux |
| gVisor sandbox (PRO) | `sandbox run --mode gvisor` | Phase 1: kernel wg0 + Phase 2: runsc | PRO + Linux |

Problems:
- The `RunscDriver` (gvisor mode) creates a **real kernel wg0** in Phase 1, which is exactly what we want to eliminate for AI agent workloads.
- Three separate code paths with divergent behaviour; the mode flag is confusing.
- Mac / non-Linux users cannot run any sandbox.
- SOCKS5 adds an unnecessary layer that leaks proxy awareness into the AI agent.

## Goal

Unify all sandbox paths into a single approach: **full gVisor userspace, no kernel wg0, works anywhere Docker runs**.

- `lattice up` is untouched — regular agents continue to use the kernel WireGuard interface.
- All AI agent sandbox use cases move to the new `lattice agent` command family.
- The `lattice sandbox` command tree is removed entirely.

## Two Scenarios

### Scenario A — Single container (Docker / local binary)

```
docker run --runtime=runsc \
  -e LATTICE_SERVER_URL=http://latticed:8080 \
  ghcr.io/alattice/lattice \
  agent run my-agent --server-url ... --token lt-xxx \
  -- python agent.py
```

The entire container runs under gVisor (`--runtime=runsc`). Inside the container:

1. `lattice agent run` registers with the control plane via NATS.
2. WireGuard uses `/dev/net/tun` — gVisor virtualises this device, so `wg0` exists only inside gVisor's userspace netstack (no kernel interface is created on the host).
3. ICE/LRP peer connections are established.
4. After `--ready-wait`, ambient capabilities are cleared, then `syscall.Exec` replaces the process with the AI agent binary.
5. The AI agent inherits the gVisor sandbox. All its `connect()` syscalls are intercepted by the gVisor sentry and routed through gVisor's netstack → virtual wg0 → WireGuard overlay.

No SOCKS5. No kernel wg0. No special routing configuration.

**Volume mounts**: if the user mounts host directories (`-v /workspace:/workspace`), the AI agent can read/write those paths. gVisor mediates access at the syscall boundary (no kernel exploit surface), but the mounted data itself is accessible. Users should mount only the directories the agent needs.

### Scenario B — Kubernetes sidecar (standard runtime, no gVisor RuntimeClass required)

```yaml
initContainers:
- name: lattice-init
  image: ghcr.io/alattice/lattice
  command: ["lattice", "agent", "init"]
  securityContext:
    capabilities:
      add: ["NET_ADMIN"]

containers:
- name: lattice-sidecar
  image: ghcr.io/alattice/lattice
  command: ["lattice", "agent", "sidecar", "my-agent",
            "--server-url", "http://latticed:8080", "--token", "lt-xxx"]
  securityContext:
    runAsUser: 1337

- name: ai-agent
  image: my-agent:latest
  # zero changes required
```

Flow:

1. **Init container** (`lattice agent init`): writes iptables REDIRECT rules — all outbound TCP from the pod is redirected to port 15001, except traffic originating from UID 1337. Then exits 0.
2. **Sidecar** (`lattice agent sidecar`, UID 1337): registers with control plane, creates a gVisor netstack using the embedded library (`gvisor.New` + `gvisor.NewTUNAdapter`), starts a transparent TCP proxy on `0.0.0.0:15001`.
3. **AI agent container**: makes TCP connections as normal. The kernel redirects them to port 15001. The sidecar reads the original destination via `SO_ORIGINAL_DST`, dials through the gVisor netstack (which has WireGuard configured), and proxies the data.
4. Sidecar's own outbound traffic (UDP WireGuard, NATS) originates from UID 1337, bypassing the iptables redirect.

No changes to the AI agent image. No `ALL_PROXY` env var required.

## CLI Design

### `lattice agent run`

```
lattice agent run <name> \
  --server-url <url> \
  --token <token> \
  [--ready-wait <duration>]    default: 3s
  [--egress-allow <CIDRs>]     PRO only; comma-separated
  [--egress-default-deny]      PRO only
  -- <command> [args...]
```

Intended as the entrypoint of a `--runtime=runsc` container. Registers, configures WireGuard, waits for peers, then `syscall.Exec`s the given command.

### `lattice agent sidecar`

```
lattice agent sidecar <name> \
  --server-url <url> \
  --token <token> \
  [--proxy-port <port>]        default: 15001
  [--egress-allow <CIDRs>]     PRO only
  [--egress-default-deny]      PRO only
```

Runs as a Kubernetes sidecar. Embeds gVisor netstack for WireGuard (no kernel wg0). Listens as a transparent TCP proxy; forwards connections through the overlay using `SO_ORIGINAL_DST`.

Must run as a UID excluded from the pod's iptables redirect rules (default UID 1337).

### `lattice agent init`

```
lattice agent init \
  [--proxy-port <port>]        default: 15001
  [--skip-uid <uid>]           default: 1337
```

Kubernetes init container. Writes iptables REDIRECT rules and exits 0. Requires `NET_ADMIN` capability. Stateless — safe to re-run.

## Internal Implementation

### File changes

**Delete**:
- `cmd/lattice/cmd/sandbox/` — entire package
- `internal/agent/runsc/` — RunscDriver two-phase orchestration, no longer needed

**Add**:
- `cmd/lattice/cmd/agent/agent.go` — top-level `lattice agent` cobra command
- `cmd/lattice/cmd/agent/run.go` + `run_community.go` — `agent run` (community: no policy)
- `cmd/lattice/cmd/agent/run_pro.go` — `agent run` with egress policy flags (PRO + linux)
- `cmd/lattice/cmd/agent/sidecar.go` + `sidecar_community.go` — `agent sidecar`
- `cmd/lattice/cmd/agent/sidecar_pro.go` — sidecar with egress policy (PRO + linux)
- `cmd/lattice/cmd/agent/init.go` — `agent init` (iptables, linux only)
- `cmd/lattice/cmd/agent/shared.go` — credential persistence (moved from sandbox_shared.go)

**Keep / reuse unchanged**:
- `internal/agent/gvisor/` — gVisor netstack library (used by sidecar mode)
- `internal/agent/sandbox_register.go` — NATS registration helpers

**Modify**:
- `cmd/lattice/cmd/root.go` — replace `sandbox.SandboxCmd()` with `agent.AgentCmd()`

### `agent run` implementation note

`sandbox_agent.go` already contains essentially the right logic: it registers via NATS, creates a WireGuard node without `CustomTUN` (so wireguard-go uses `/dev/net/tun`, which gVisor virtualises), and `syscall.Exec`s the AI agent as PID 1. The new `agent run` is a direct lift of this logic, promoted to a first-class command and stripped of the `--network=host` dependency. The outer `RunscDriver` (which created the kernel wg0 in Phase 1) is deleted entirely.

### `agent sidecar` implementation note

The transparent proxy listener reads `SO_ORIGINAL_DST` on each accepted connection to retrieve the original destination before the iptables REDIRECT. It then dials that destination through the gVisor netstack (which has WireGuard peers configured), and copies data bidirectionally. The gVisor netstack setup is identical to the existing `PodDriver` in `driver_pod.go`, minus the SOCKS5 server.

## PRO / Community split

| Feature | Community | PRO |
|---------|-----------|-----|
| `agent run` | All egress allowed | + `--egress-allow` / `--egress-default-deny` enforced via gVisor virtual iptables |
| `agent sidecar` | All egress allowed | + egress policy enforced at transparent proxy layer |
| `agent init` | Available | Available |

Build tags follow the existing pattern: `//go:build pro && linux` for PRO-only files, `//go:build !pro` for community stubs.

## Out of Scope

- **Filesystem access controls**: volume mount permissions are the user's responsibility at `docker run -v` / Pod spec level. A `--mount` whitelist flag is a candidate for a future PRO enhancement.
- **K8s with gVisor RuntimeClass**: if the cluster has `runtimeClassName: gvisor` available, the sidecar + iptables approach is unnecessary (all containers share gVisor's netstack). This is a documented alternative, not a primary target for this change.
- **Windows / WSL2**: supported via Docker Desktop (`--runtime=runsc` requires gVisor on the Docker Desktop VM). No code changes required.
