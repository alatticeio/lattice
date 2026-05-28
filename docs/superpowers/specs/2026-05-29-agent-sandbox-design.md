# AI Agent Sandbox Design

**Date**: 2026-05-29
**Status**: Implemented (Phase 1)
**Replaces**: 2026-05-28-agent-sandbox-design.md, 2026-05-28-tier-based-features-design.md

## Goal

Single binary, zero build tags. Features enabled at runtime based on the user's
account tier returned by the server during registration.

| Edition | Network isolation | Egress control | Audit log | How |
|---------|:---:|:---:|:---:|---|
| **Community** | ✅ gVisor netstack + WireGuard | ❌ | ❌ | tier="community" |
| **PRO** | ✅ gVisor netstack + WireGuard | ✅ PolicyChecker | ✅ AuditWriter | tier="pro" |

**Same binary, same `docker run` command. Different behavior controlled by server.**

## Architecture

```
docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice \
  sandbox run my-agent --server-url ... --token ... \
  [--egress-allow CIDR] [--egress-default-deny] \
  -- python agent.py

sandbox run:
  1. registerOrResume → NATS → server returns { tier: "pro"|"community" }
  2. if tier == "pro": PolicyChecker + AuditWriter
     if tier == "community" && egress flags: warning, flags ignored
  3. gVisor netstack + WireGuard CustomTUN (no kernel wg0)
  4. iptables REDIRECT → port 15001 (exempt UID 0, redirect UID 999)
  5. tproxy listener (SO_ORIGINAL_DST → original destination)
  6. Wait for WireGuard peer sessions
  7. fork AI agent as UID 999

AI agent connect() → iptables REDIRECT → tproxy → gVisor netstack
                                                    │
                                          ┌─────────┴──────────┐
                                          │  Community: pass     │
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
- **Single binary**: tier determines features, not build tags

## Tier Mechanism

Follows the existing `EnforcerMode` pattern:

```
Server                          NATS                          Agent
─────                           ────                          ─────

UserProfile.Tier = "pro"
        │
RegisterAgent
  ├─ Lookup workspace → owner
  ├─ Lookup user profile
  ├─ Read profile.Tier
  └─ return { tier: "pro" }  ──→ infra.Peer.Tier ──→ currentPeer.Tier
                                                        │
                                      sandbox run → PolicyChecker? AuditWriter?
                                      enforcer    → eBPF available?
                                      telemetry   → system scrapers?
```

### Server Changes

| File | Change |
|------|--------|
| `internal/server/models/user.go` | `Tier string` field (default `"community"`) |
| `internal/server/dto/user.go` | `Tier` in profile req/resp |
| `internal/server/service/user_profile.go` | Read/write `Tier` |
| `internal/server/service/agent_registration.go` | Return `Tier` in response |
| `internal/server/server/server.go` | Pass `Tier` to `infra.Peer` |

### Agent Changes

| File | Change |
|------|--------|
| `internal/agent/infra/message.go` | `Peer.Tier` field |
| `cmd/lattice/cmd/sandbox/run.go` | Read `peer.Tier` after registration, build policy/audit |

## File Layout

```
cmd/lattice/cmd/sandbox/
├── agent.go          # SandboxCmd() entry point
├── run.go            # Unified CLI (//go:build linux)
│                     #   Always registers --egress-allow / --egress-default-deny
│                     #   Reads peer.Tier to decide policy/audit
├── run_stub.go       # Non-Linux stub (//go:build !linux)
├── shared.go         # Credential persistence, overlayAddr
├── shared_linux.go   # runSandbox engine + helpers
│                     #   forkAndWait (UID 999), installRunIPTables,
│                     #   runPeriodicRefresh, fileAuditWriter
├── sidecar.go        # Unified sidecar (//go:build linux)
│                     #   Reads peer.Tier to decide policy/audit
├── sidecar_stub.go   # Non-Linux stub (//go:build !linux)
├── init.go           # buildIPTablesRules (//go:build linux)
├── init_stub.go      # Non-Linux stub
└── *_test.go
```

**Removed** (Phase 1 — no more `//go:build pro`):

| File | Reason |
|------|--------|
| `run_community.go` | Merged into `run.go` |
| `run_pro.go` | Merged into `run.go` |
| `run_pro_stub.go` | No longer needed |

## Egress Policy & Audit (PRO)

### PolicyChecker

```bash
# Pro account — enforced
lattice sandbox run agent-1 \
  --egress-allow 10.0.0.0/8 --egress-default-deny \
  -- python agent.py

# Community account — warning
lattice sandbox run agent-1 \
  --egress-allow 10.0.0.0/8 -- python agent.py
# WARNING: egress policy flags require a Pro account. These flags will be ignored.
```

Every `connect()` checked against allowed CIDRs before TCP handshake.
Denied = immediate error returned to agent.

### AuditWriter

All connections logged as JSON to `/tmp/lattice-audit.jsonl`:

```json
{"identity":"agent-1","network":"tcp","addr":"10.100.0.5:8080","allowed":true}
{"identity":"agent-1","network":"tcp","addr":"1.2.3.4:443","allowed":false}
```

## Docker Usage

```bash
# Community
docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice \
  sandbox run my-agent --server-url http://latticed:8080 --token lt-xxx \
  -- python agent.py

# PRO (same command, egress flags respected)
docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice \
  sandbox run my-agent --server-url http://latticed:8080 --token lt-xxx \
  --egress-allow 10.0.0.0/8 --egress-default-deny \
  -- python agent.py
```

`--cap-add NET_ADMIN` is required for iptables (transparent interception).
For full syscall isolation, users add `--runtime=runsc` at the Docker level.

## Unified Tier — All Phases

| Phase | System | Status |
|:---:|------|:---:|
| 1 | Sandbox egress/audit | ✅ Done |
| 2 | Sandbox sidecar | ✅ Done |
| 3 | Enforcer (eBPF/iptables) | ✅ Done |
| 4 | Telemetry (system/WG/ICMP) | 🔲 (heavy deps, build tags remain) |
| 5 | Server (dashboard/monitor/audit/SSO/intent) | ✅ Done (monitor/dashboard/audit merged; dex/intent build tags remain) |
| 6 | TURN relay | ✅ Deleted (using external coturn) |
| 7 | License | ✅ Done |

## Out of Scope

- Full syscall sandbox (Docker `--runtime=runsc` provides this externally)
- Filesystem access controls
- License key enforcement (trust-based tier for now)
