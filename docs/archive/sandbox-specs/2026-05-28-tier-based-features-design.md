# Tier-Based Feature Activation Design

**Date**: 2026-05-29
**Status**: Draft

## Goal

Eliminate build-time PRO/community separation. Single binary, features enabled
at runtime based on the user's account tier returned by the server during
registration.

## Prior Art

`EnforcerMode` already follows this pattern:

```
1. User sets enforcer_mode in profile settings (server)
2. Server returns EnforcerMode in registration response
3. Agent reads current.EnforcerMode after registration
4. Agent selects iptables or ebpf backend
```

We add a `Tier` field that follows the same flow.

## Design

```
                    ┌─────────────────────────┐
                    │  Server                  │
                    │  UserProfile.Tier        │
                    │    → "community" | "pro" │
                    │                          │
                    │  Registration response:   │
                    │    { tier: "pro" }        │
                    └──────────┬──────────────┘
                               │
                    ┌──────────▼──────────────┐
                    │  Agent (single binary)   │
                    │                          │
                    │  sandbox run ...         │
                    │    --egress-allow ...    │ ← always available
                    │    --egress-default-deny │
                    │                          │
                    │  if tier == "pro":       │
                    │    PolicyChecker ✅       │
                    │    AuditWriter ✅         │
                    │  else:                   │
                    │    policyChecker = nil   │
                    │    auditWriter = nil     │
                    │    warn if egress flags  │
                    │      used without tier   │
                    └──────────────────────────┘
```

## Server Changes

### User Model

Add `Tier` field alongside existing `EnforcerMode`:

```go
// internal/server/models/user.go
type User struct {
    // ... existing fields ...
    EnforcerMode string `gorm:"size:16;default:'auto'" json:"enforcerMode"`
    Tier         string `gorm:"size:16;default:'community'" json:"tier"` // new
}
```

### Profile DTO + API

Update `internal/server/dto/user.go` to include `Tier` in read/write.

### Registration Response

Add `Tier` to `RegisterAgentResult` in `internal/server/service/agent_registration.go`:

```go
type RegisterAgentResult struct {
    // ... existing fields ...
    EnforcerMode string `json:"enforcerMode,omitempty"`
    Tier         string `json:"tier,omitempty"` // new: "community" | "pro"
}
```

Read from user profile:

```go
var userTier string
if profile, profErr := s.profileService.GetByUserID(ctx, userID); profErr == nil {
    if profile.Tier != "" {
        userTier = profile.Tier
    }
}
result.Tier = userTier
```

## Agent Changes

### Peer struct

Add `Tier` to `infra.Peer`:

```go
type Peer struct {
    // ... existing ...
    EnforcerMode string `json:"enforcerMode,omitempty"`
    Tier         string `json:"tier,omitempty"` // new
}
```

### Read tier after registration

In `node.go`, after registration, read tier (same pattern as EnforcerMode):

```go
if node.current.Tier != "" {
    // store for sandbox use
}
```

### Sandbox CLI — single binary

Merge `run_community.go` + `run_pro.go` into a single `run.go`:

- `//go:build linux` (no pro build tag)
- Always registers `--egress-allow` and `--egress-default-deny` flags
- After registration, checks tier:
  - `"pro"` → creates PolicyChecker + AuditWriter
  - `"community"` → policyChecker=nil, auditWriter=nil
  - If user passed egress flags without pro tier → warn and proceed without enforcement

```go
func runRun(cmd *cobra.Command, args []string) error {
    // ... parse args ...

    // Register → get tier from currentPeer.Tier
    currentPeer, err := registerOrResume(...)

    var policyChecker shim.PolicyChecker
    var auditWriter shim.AuditWriter

    if currentPeer.Tier == "pro" {
        policyChecker = buildEgressPolicy()
        auditWriter = newFileAuditWriter(auditLogPath)
    } else if runEgressDeny || runEgressAllow != "" {
        fmt.Fprintln(os.Stderr, "[sandbox-run] WARNING: egress policy flags require a Pro account. These flags will be ignored.")
    }

    return runSandbox(ctx, cancel, agentName, ..., policyChecker, auditWriter, cmdArgs)
}
```

## Build Tag Cleanup

| File | Before | After |
|------|--------|-------|
| `run_community.go` | `!pro && linux` | Delete, merge into run.go |
| `run_pro.go` | `pro && linux` | Delete, merge into run.go |
| `run_community_stub.go` | `!pro && !linux` | Rename to run_stub.go, `!linux` |
| `run_pro_stub.go` | `pro && !linux` | Delete |
| `run.go` | — | New, `linux` |

Also remove build tags from:
- `sidecar_community.go` → `sidecar.go` (always compiled on linux)
- `sidecar_pro.go` → merge into `sidecar.go` with tier check
- `sidecar_pro_stub.go` → delete

(These can be separate PRs — sandbox run first.)

## User Experience

```bash
# Community user — egress flags silently ignored
$ lattice sandbox run agent-1 --egress-allow 10.0.0.0/8 -- python agent.py
[sandbox-run] WARNING: egress policy flags require a Pro account. These flags will be ignored.

# Pro user — egress enforced
$ lattice sandbox run agent-1 --egress-allow 10.0.0.0/8 --egress-default-deny -- python agent.py
[sandbox-run] egress policy: deny-default, allow=[10.0.0.0/8]
[sandbox-run] audit: /tmp/lattice-audit.jsonl

# Both use the SAME binary.
```

## Out of Scope

- License enforcement (tier is trust-based; enterprise license keys come later)
- Dynamic tier changes mid-session (checked once at registration)
