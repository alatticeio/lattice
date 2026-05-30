# AI Agent Sandbox Design

**Date**: 2026-05-29
**Status**: Implemented
**Repos**: lattice, lattice-shim

## Goal

Provide transparent WireGuard overlay networking for AI agents with optional
egress policy control and audit logging. Both editions share the same core
architecture; PRO adds observability and access control hooks.

| Edition | Network isolation | Egress control | Audit log | User command |
|---------|:---:|:---:|:---:|---|
| **Community** | ✅ gVisor netstack + WireGuard | ❌ | ❌ | `docker run --rm --cap-add NET_ADMIN` |
| **PRO** | ✅ gVisor netstack + WireGuard | ✅ PolicyChecker | ✅ AuditWriter | `docker run --rm --cap-add NET_ADMIN` |

## Architecture

Both editions use the same transparent interception architecture:

```
docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice \
  sandbox run my-agent --server-url ... --token ... -- python agent.py

sandbox run:
  1. registerOrResume → NATS registration, get overlay IP + peers
  2. gVisor netstack + WireGuard CustomTUN (no kernel wg0)
  3. iptables REDIRECT → port 15001 (exempt UID 0, redirect UID 999)
  4. tproxy listener (SO_ORIGINAL_DST recovers original destination)
  5. Wait for WireGuard peer sessions
  6. fork AI agent as UID 999

AI agent connect() → iptables REDIRECT → tproxy → gVisor netstack
                                                    │
                                          ┌─────────┴──────────┐
                                          │  Community: 空        │
                                          │  PRO: PolicyChecker   │
                                          │       AuditWriter     │
                                          └──────────────────────┘
                                                    │
                                          CustomTUN → WireGuard → overlay
```

Key properties:
- **Agent-zero-config**: no SOCKS5, no proxy env vars, no code changes
- **No kernel wg0**: WireGuard runs on gVisor userspace netstack via CustomTUN
- **Transparent interception**: iptables REDIRECT + SO_ORIGINAL_DST

## Community vs PRO

The core sandbox engine (`runSandbox` in `shared_linux.go`) is identical.
The only difference is what gets injected at netstack creation time:

```go
// Community (run_community.go):
runSandbox(ctx, cancel, name, serverURL, token,
    nil,  // policyChecker — allows everything
    nil,  // auditWriter  — silent
    cmdArgs)

// PRO (run_pro.go):
policyChecker := shimfwd.NewEgressFilter(egressPolicy)
auditWriter, _ := newFileAuditWriter("/tmp/lattice-audit.jsonl")
runSandbox(ctx, cancel, name, serverURL, token,
    policyChecker,  // enforces --egress-allow / --egress-default-deny
    auditWriter,    // writes JSON lines to audit log
    cmdArgs)
```

## PRO: Egress Control & Audit

### PolicyChecker (egress CIDR allow/deny)

```bash
# Only allow agent to reach the 10.0.0.0/8 overlay range:
lattice sandbox run agent-1 \
  --egress-allow 10.0.0.0/8 --egress-default-deny \
  -- python agent.py
```

Every `connect()` is checked against the allowed CIDRs before the TCP
handshake completes. Denied connections return immediately with an error.

### AuditWriter

Every connection is logged as JSON to `/tmp/lattice-audit.jsonl`:

```json
{"identity":"agent-1","network":"tcp","addr":"10.100.0.5:8080","allowed":true}
{"identity":"agent-1","network":"tcp","addr":"1.2.3.4:443","allowed":false}
```

## File Layout

| File | Build tag | Purpose |
|------|-----------|---------|
| `shared_linux.go` | `linux` | `runSandbox` core engine + helpers |
| `run_community.go` | `!pro && linux` | CLI definition, no egress flags, nil policy/audit |
| `run_community_stub.go` | `!pro && !linux` | Empty `addRunCmd` for non-Linux |
| `run_pro.go` | `pro && linux` | CLI definition, egress flags, PolicyChecker + AuditWriter |
| `run_pro_stub.go` | `pro && !linux` | Empty `addRunCmd` for non-Linux |
| `init.go` | `linux` | `buildIPTablesRules` (reused by shared_linux.go) |
| `shared.go` | all | Credentials persistence, `overlayAddr` |

## Docker Usage

```bash
# Community
docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice \
  sandbox run my-agent --server-url http://latticed:8080 --token lt-xxx \
  -- python agent.py

# PRO (same command, just adds --egress-* flags)
docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice \
  sandbox run my-agent --server-url http://latticed:8080 --token lt-xxx \
  --egress-allow 10.0.0.0/8 --egress-default-deny \
  -- python agent.py
```

`--cap-add NET_ADMIN` is required for iptables in both editions.
For full syscall isolation, users can add `--runtime=runsc` at the Docker
level (syscall = Docker's concern, network = Lattice's concern).

## Out of Scope

- Full syscall sandbox (Docker `--runtime=runsc` provides this externally)
- Filesystem access controls
- Custom capability sets per agent
