# Unified Tier Architecture: Single Binary, Account-Based Features

**Date**: 2026-05-29
**Status**: Draft

## Goal

Eliminate all `//go:build pro` build tags. One binary. Features activated at
runtime based on the user's account tier returned by the server during
registration.

## Current State

```
Build time (two binaries):
  community: go build    → 无 PRO 功能
  pro:       go build -tags pro → 全功能

Runtime (static):
  编译时写死的功能，不能动态切换
```

## Target State

```
Build time (one binary):
  lattice: go build      → 包含全部功能代码

Runtime (dynamic):
  Server 返回 tier → "community" | "pro"
  Agent 读取 tier → 启用/禁用功能模块
```

## Tier Model

```go
// internal/server/models/user.go
type Tier string

const (
    TierCommunity Tier = "community"  // default
    TierPro       Tier = "pro"
)
```

## Feature Categories

All current PRO-only features mapped to tier:

### Category 1: Network Policy & Audit (Sandbox)

| Feature | Community | Pro |
|---------|:---:|:---:|
| gVisor netstack + WireGuard | ✅ | ✅ |
| iptables REDIRECT + tproxy | ✅ | ✅ |
| egress CIDR allow/deny (`--egress-allow`) | ⚠️ flag exists, ignored | ✅ PolicyChecker |
| audit log (`/tmp/lattice-audit.jsonl`) | ❌ | ✅ AuditWriter |
| TURN relay | ❌ | ✅ |

### Category 2: eBPF Enforcement

| Feature | Community | Pro |
|---------|:---:|:---:|
| iptables enforcer | ✅ (default) | ✅ |
| eBPF TC enforcer | ❌ | ✅ |
| `--enforcer-mode` flag | ✅ | ✅ |

### Category 3: Observability

| Feature | Community | Pro |
|---------|:---:|:---:|
| Prometheus metrics | ✅ | ✅ |
| System telemetry (CPU/mem/disk) | ❌ | ✅ |
| WireGuard interface telemetry | ❌ | ✅ |
| ICMP latency scraper | ❌ | ✅ |
| Dashboard (latticed) | ❌ | ✅ |
| Audit consumer | ❌ | ✅ |
| Monitor | ❌ | ✅ |

### Category 4: Platform

| Feature | Community | Pro |
|---------|:---:|:---:|
| SSO / Dex / OIDC login | ❌ | ✅ |
| Intent engine | ❌ | ✅ |
| License validation | ❌ | ✅ |
| TURN relay (LRP) | ❌ | ✅ |
| K8s sidecar egress | ❌ | ✅ |

## Implementation Pattern

Every PRO feature follows the same pattern:

### Step 1: Merge community/pro files

```go
// Before:
//   enforcer_selector_community.go  (//go:build !pro)
//   enforcer_selector_pro.go        (//go:build pro)
//
// After:
//   enforcer_selector.go            (no build tag)

func SelectEnforcerMode(cfg *Config, tier Tier, logger *log.Logger) EnforcerMode {
    switch cfg.EnforcerMode {
    case "ebpf":
        if tier != TierPro {
            logger.Warn("eBPF enforcer requires Pro account, falling back to iptables")
            return ModeIptables
        }
        if selectEBPFAvailable() {
            return ModeEBPF
        }
        logger.Warn("eBPF not available on this system, falling back to iptables")
        return ModeIptables
    case "iptables":
        return ModeIptables
    default: // "auto"
        if tier == TierPro {
            if selectEBPFAvailable() {
                return ModeEBPF
            }
        }
        return ModeIptables
    }
}
```

### Step 2: Pass tier from registration to feature gates

```go
// Registration response carries tier
type RegisterAgentResult struct {
    // ... existing fields ...
    Tier string `json:"tier,omitempty"`
}

// Agent stores tier after registration
type Peer struct {
    // ... existing fields ...
    Tier string `json:"tier,omitempty"`
}

// Feature gate helper
func (p *Peer) IsPro() bool {
    return p.Tier == "pro"
}
```

### Step 3: Server returns tier

```go
// internal/server/service/agent_registration.go
func (s *Service) register(ctx context.Context, tok *Token, ...) (*RegisterAgentResult, error) {
    // ... existing token validation ...

    // Get user profile → tier
    var userTier string
    if profile, err := s.profileService.GetByUserID(ctx, userID); err == nil {
        userTier = profile.Tier
    }

    return &RegisterAgentResult{
        // ... existing ...
        Tier: userTier,
    }, nil
}
```

## Feature Gate Helper

A single, centralized helper for all feature decisions:

```go
// pkg/features/features.go

package features

type Tier string

const (
    TierCommunity Tier = "community"
    TierPro       Tier = "pro"
)

// Gate controls all tier-dependent features.
type Gate struct {
    Tier Tier
}

func NewGate(tier string) *Gate {
    t := TierCommunity
    if tier == "pro" {
        t = TierPro
    }
    return &Gate{Tier: t}
}

func (g *Gate) IsPro() bool      { return g.Tier == TierPro }
func (g *Gate) String() string    { return string(g.Tier) }
```

## Top-Level Flow

```
┌─────────────────────────────────────────────────┐
│  Server                                          │
│                                                  │
│  User ──→ UserProfile.Tier                       │
│              │                                   │
│  Token ──→ AgentRegistration                     │
│              │                                   │
│              ├── Lookup user by token            │
│              ├── Read profile.Tier               │
│              └── Return { tier: "pro", ... }     │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│  Agent (single binary)                           │
│                                                  │
│  Register → Peer.Tier = "pro"                    │
│                                                  │
│  features.NewGate(peer.Tier)                     │
│       │                                          │
│       ├── sandbox: PolicyChecker? Audit?         │
│       ├── enforcer: ebpf?                        │
│       ├── telemetry: system scraper?             │
│       └── TURN: relay enabled?                   │
└──────────────────────────────────────────────────┘
```

## Migration Strategy

### Phase 1: Sandbox (first, smallest)

| Files merged | ~6 files → ~2 files |
| Build tags removed | `pro && linux`, `!pro && linux` |

### Phase 2: Enforcer + eBPF

| Files merged | `enforcer_selector_pro.go` + `enforcer_selector_community.go` |
| | `manager_pro.go` + `manager_community.go` |

### Phase 3: Telemetry (3 subsystems)

### Phase 4: Server-side (dashboard, monitor, audit, SSO, intent)

### Phase 5: TURN relay

### Phase 6: License

## Out of Scope

- License key enforcement (separate mechanism, trust-based for now)
- Runtime tier changes without re-registration
- Per-feature billing (tier is coarse-grained: community | pro)
