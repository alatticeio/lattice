# Tier-Based Feature Activation: Phase 2-7 Design

**Date**: 2026-05-29
**Status**: Draft
**Phase 1**: ✅ Complete (sandbox run)
**Phase 2**: ✅ Complete (sandbox sidecar)
**Phase 3**: ✅ Complete (enforcer — tier-gated eBPF)
**Phase 5**: ✅ Partial (monitor/dashboard/audit merged; dex/intent build tags remain)
**Phase 6**: ✅ Deleted (TURN relay removed — using external coturn)
**Phase 7**: ✅ Complete (license — tier-aware verifier)
**Depends on**: `2026-05-29-agent-sandbox-design.md`

## Goal

Eliminate all remaining `//go:build pro` build tags. One binary, all features.
Activation controlled by `currentPeer.Tier` ("community" | "pro") from server
registration response.

## Common Pattern

Every phase follows the same three-step pattern:

```
1. Merge community_stub.go + pro.go → single file (no build tag)
2. Gate PRO behavior behind tier check
3. Remove build tag, delete stub file
```

## Phase 2: Sandbox Sidecar

**Files to merge**:

| Before | After |
|--------|-------|
| `sidecar_community.go` (`!pro`) | → `sidecar.go` |
| `sidecar_pro.go` (`pro && linux`) | → (merged into sidecar.go) |
| `sidecar_pro_stub.go` (`pro && !linux`) | → delete |

**PRO behavior**: `--egress-allow` / `--egress-default-deny` flags, `PolicyChecker` + `AuditWriter` injected into gVisor sandbox config.

**Community behavior**: same gVisor netstack + tproxy, no policy/audit.

**Tier gate**:

```go
func runSidecar(cmd *cobra.Command, args []string) error {
    // ... register → get peer ...

    var policyChecker shim.PolicyChecker
    var auditWriter shim.AuditWriter

    if peer.Tier == "pro" {
        if sidecarEgressDeny || sidecarEgressAllow != "" {
            policyChecker = shim.NewEgressFilter(buildEgressPolicy())
            auditWriter, _ = newFileAuditWriter(auditLogPath)
        }
    } else if sidecarEgressDeny || sidecarEgressAllow != "" {
        fmt.Fprintln(os.Stderr, "WARNING: egress flags require Pro account")
    }

    // ... rest is identical for both tiers ...
}
```

## Phase 3: Enforcer (eBPF)

**Files to merge**:

| Before | After |
|--------|-------|
| `internal/agent/provision/enforcer_selector_community.go` (`!pro`) | → (merge into selector.go) |
| `internal/agent/provision/enforcer_selector_pro.go` (`pro`) | → (merge into selector.go) |
| `internal/agent/ebpf/manager_community.go` (`!pro`) | → (merge into manager.go) |
| `internal/agent/ebpf/manager_pro.go` (`pro && linux`) | → (merge into manager.go) |

**PRO behavior**: Probe eBPF availability, load TC ingress BPF program, provision firewall rules into BPF maps.

**Community stub**: `selectEBPFAvailable()` returns `ModeIPTables`. eBPF Manager returns stub with "Pro feature" error.

**Tier gate**:

```go
func SelectEnforcerMode(cfg *Config, tier string, logger *log.Logger) EnforcerMode {
    switch cfg.EnforcerMode {
    case "ebpf":
        if tier != "pro" {
            logger.Warn("eBPF requires Pro account, falling back to iptables")
            return ModeIptables
        }
        if selectEBPFAvailable() { return ModeEBPF }
        return ModeIptables
    case "iptables":
        return ModeIptables
    default: // "auto"
        if tier == "pro" && selectEBPFAvailable() { return ModeEBPF }
        return ModeIptables
    }
}
```

**Note**: This phase pulls in `github.com/cilium/ebpf` as unconditional dependency.

## Phase 4: Telemetry

**Files to merge**:

| Before | After |
|--------|-------|
| `internal/telemetry/collector.go` (`pro`) | → remove tag |
| `internal/telemetry/scraper_system.go` (`pro`) | → remove tag |
| `internal/telemetry/scraper_wireguard.go` (`pro`) | → remove tag |
| `internal/telemetry/scraper_icmp.go` (`pro`) | → remove tag |
| `internal/telemetry/telemetry_community.go` (`!pro`) | → delete |

**PRO behavior**: `New()` creates Collector with 3 scrapers (system/WireGuard/ICMP), pushes to VictoriaMetrics via Prometheus Remote Write.

**Community stub**: `New()` returns error "telemetry push is a Lattice Pro feature".

**Tier gate**: Caller checks tier before calling `telemetry.New()`:

```go
// In agent startup code:
if features.IsPro(tier) {
    collector, err := telemetry.New(cfg)
    if err != nil { /* log, non-fatal */ }
    go collector.Run(ctx)
}
```

**Note**: Pulls in `gopsutil`, `pro-bing`, `VictoriaMetrics/metrics` as unconditional deps.

## Phase 5: Server Features

### 5a. Monitor

**Files to merge**: `monitor.go` + `monitor_community.go`

**Tier gate**: merge routers into one. Community route returns 402 via existing `requireFeature` middleware. The middleware checks runtime tier instead of build-time license.

### 5b. Dashboard

**Files to merge**: `dashboard.go` + `dashboard_community.go`

**Tier gate**: same pattern as monitor. Merge routers, keep `requireFeature`.

### 5c. Audit Consumer

**Files to merge**: `audit_consumer_pro.go` + `audit_consumer_community.go`

**Tier gate**: `initFlowAuditConsumer()` checks tier. If pro, creates NATS subscription + persists FlowEvents. If community, returns nil (no-op).

### 5d. Dex OIDC / SSO

**Files to merge**: `internal/server/dex/login.go` + `dex_community.go`

**Tier gate**: merge both files. `NewDex()` returns error if tier != pro. `Login()` returns 402 for community.

### 5e. Intent Engine

**Files to merge**: `internal/server/service/intent_pro.go` + `intent_community.go`

**Tier gate**: merge both files. All methods return `ErrPaymentRequired` if tier != pro.

**Note**: Pulls in LLM client dependency.

## Phase 6: TURN Relay — Deleted

All TURN code removed. Using external coturn for STUN service.

**Deleted files**: `internal/relay/turn_server.go`, `turn_client.go`, `turn_run.go`, `turn_auth.go`, `turn_community.go`, `internal/turn.go`, `cmd/manager/cmd/turn.go`

**Renamed**: `TurnServerURL` → `StunServerURL`, `TurnServerDomain` → `StunServerDomain`, `DefaultTurnServerPort` → `DefaultStunServerPort`

**Removed dependency**: `github.com/pion/turn/v4`

## Phase 7: License

**Files to merge**:

| Before | After |
|--------|-------|
| `internal/license/validator.go` (`pro`) | → remove tag |
| `internal/license/features.go` (`pro`) | → remove tag |
| `internal/license/keys_pro.go` (`pro`) | → remove tag |
| `internal/license/validator_community.go` (`!pro`) | → delete |
| `internal/license/features_community.go` (`!pro`) | → delete |

**Tier gate**: License becomes an optional overlay for air-gapped deployments.

```go
func NewVerifier(tier string) Verifier {
    if tier == "pro" {
        // Try license file first, fall back to tier-based
        return newProVerifier()
    }
    return &communityVerifier{tier: tier}
}

func (v *proVerifier) HasFeature(feature string) bool {
    // Check JWT license claims first
    if v.license.HasFeature(feature) { return true }
    // Fall back to tier-based: pro tier grants all features
    return v.tier == "pro"
}
```

## Dependency Impact

| Phase | New unconditional dependency |
|:---:|------|
| 3 | `github.com/cilium/ebpf` |
| 4 | `github.com/shirou/gopsutil/v4`, `github.com/prometheus-community/pro-bing`, `github.com/VictoriaMetrics/metrics` |
| 5 | (already in `go.mod` via server) |
| 6 | ~~deleted~~ (TURN removed, using external coturn) |
| 7 | (already in `go.mod`) |

Binary size increases ~5-10MB from these deps. Acceptable for a single-binary distribution.

## Migration Order

```
Phase 2 (sidecar)     ← done
Phase 3 (enforcer)    ← done
Phase 4 (telemetry)   ← adds gopsutil + pro-bing deps
Phase 5 (server)      ← mostly route merging
Phase 6 (TURN)        ← deleted (external coturn)
Phase 7 (license)     ← done
```

Phases 2-7 are mostly independent — can be done in any order after Phase 1.
