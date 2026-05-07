# OpenCore Licensing Design

**Date**: 2026-04-30
**Status**: Draft v2 (revised after review)
**Author**: Claude Code

## Context

Lattice is a Cloud-Native WireGuard Network Orchestration platform. The business model is OpenCore: core features open-source (Apache 2.0), enterprise features under a proprietary license for self-hosted commercial licensing.

**Business Strategy**:
- Short-term: Self-hosted licensing only
- Long-term: SaaS offering after revenue established
- Target market: All segments (SMB, enterprise, MSP, developer/individual)
- Revenue model: Multi-tier self-hosted license subscriptions
- Distribution: Pro binary publicly downloadable (GitHub Releases); license file controls feature access
- License delivery: Fully automated self-service portal (purchase → generate → download → install)

## Architecture

### Code Organization

Single repository with Go build tags (`community` vs `pro`). Community binary contains zero enterprise code. This extends the existing pattern already in place with 5 community stubs.

```
internal/
├── server/
│   ├── license/              # NEW: License verification
│   │   ├── license.go        # Core verification interface (shared)
│   │   ├── validator.go      # Ed25519 JWT offline verification
│   │   ├── heartbeat.go      # Optional online heartbeat client
│   │   ├── storage.go        # License file loading (multi-path)
│   │   └── models.go         # License structure definitions
│   ├── middleware/
│   │   └── license_check.go  # HTTP middleware: intercept unauthorized access
│   ├── dex/
│   │   ├── login.go              # PRO: OIDC login implementation
│   │   └── dex_community.go      # Community stub
│   └── server/
│       ├── dashboard.go          # PRO: Analytics dashboard
│       ├── dashboard_community.go # Community stub
│       ├── monitor.go            # PRO: Monitoring
│       └── monitor_community.go  # Community stub
├── relay/
│   ├── turn_server.go           # PRO: TURN server
│   └── turn_community.go        # Community stub
├── telemetry/
│   ├── collector.go             # PRO: VictoriaMetrics collector
│   ├── scraper_*.go             # PRO: Various scrapers
│   └── telemetry_community.go   # Community stub
└── agent/                       # Fully open-source, enterprise features controlled server-side
```

### Design Principles

- All enterprise features paired as `*_community.go` (stub) and counterpart (Pro implementation)
- Community stubs return HTTP 402 or gRPC `UNIMPLEMENTED`
- License validation at API entry via unified middleware
- Core network features never degrade even when license expires
- Heartbeat is always optional; air-gapped deployments are first-class citizens

## Feature Matrix

| Feature | Community | Standard | Enterprise |
|---------|-----------|----------|------------|
| **Core Networking** | | | |
| WireGuard P2T tunnels | ✅ | ✅ | ✅ |
| ICE NAT traversal | ✅ | ✅ | ✅ |
| Basic IPAM | ✅ | ✅ | ✅ |
| Network policies (ALLOW/DENY) | ✅ | ✅ | ✅ |
| K8s Operator (CRD) | ✅ | ✅ | ✅ |
| DNS internal resolution | ✅ | ✅ | ✅ |
| **Relay** | | | |
| Relay config/registration | ✅ | ✅ | ✅ |
| Relay server runtime (LRP) | ✅ | ✅ | ✅ |
| Relay dashboard (status/usage) | list only | ✅ charts | ✅ history analysis |
| Relay alerts/notifications | ❌ | ✅ | ✅ |
| Relay auto-scaling | ❌ | ❌ | ✅ |
| TURN server | ❌ | ✅ | ✅ |
| **Frontend UI** | | | |
| Node management | ✅ | ✅ | ✅ |
| Workspace management | ✅ | ✅ | ✅ |
| Network topology view | ✅ | ✅ | ✅ |
| Policy management | ✅ | ✅ | ✅ |
| Token management | ✅ | ✅ | ✅ |
| Peering management | ✅ | ✅ | ✅ |
| Cluster peering | ✅ | ✅ | ✅ |
| Dashboard basic panel | ✅ | ✅ | ✅ |
| **Identity/Access** | | | |
| Basic auth (API Key) | ✅ | ✅ | ✅ |
| OIDC/SSO (Dex) | ❌ | ✅ | ✅ |
| Basic RBAC | ✅ (admin/user) | ✅ | ✅ (fine-grained) |
| Advanced RBAC (scope/role) | ❌ | ❌ | ✅ |
| Approval workflow | ❌ | ❌ | ✅ |
| Member management | ✅ | ✅ | ✅ |
| **Operations** | | | |
| Monitoring/telemetry panel | ❌ | ✅ | ✅ |
| Audit logging | ❌ | ✅ | ✅ |
| Alert rules | ❌ | ✅ | ✅ |
| VictoriaMetrics integration | ❌ | ✅ | ✅ |
| Grafana panels | ❌ | ❌ | ✅ |
| Webhook integration | ❌ | ❌ | ✅ |
| SIEM integration | ❌ | ❌ | ✅ |
| **Enterprise Features** | | | |
| AI security audit | ❌ | ❌ | ✅ |
| Multi-cluster unified control | ❌ | ❌ | ✅ |
| License management UI | ❌ | ✅ | ✅ |
| Billing/subscription management | ❌ | ✅ | ✅ |
| Notification system | ❌ | ✅ | ✅ |
| Support | ❌ | Email | Phone+email+SLA |

**Note on Relay dashboard (Community)**: Community edition returns relay list (name, status, endpoint) via API. Charts, usage history, and analytics require Standard or above. Community stubs for chart/analytics endpoints return HTTP 402.

## License System

### License Format: Ed25519 JWT

License files use JWT format (RFC 7519) with Ed25519 algorithm (OKP, `EdDSA`). This is the industry standard used by Teleport, Grafana, and others. The three-part `header.payload.signature` format is self-contained and harder to tamper with than a JSON + separate signature approach.

Library: `github.com/golang-jwt/jwt/v5` with `ed25519.PublicKey` verifier.

### License Types

| Type | Use Case | Validation | Grace Period | Heartbeat |
|------|----------|-----------|-------------|-----------|
| **Trial** | 30-day evaluation | Offline JWT only | 0 days | Not required |
| **Standard** | Single-cluster annual | Offline JWT + optional heartbeat | 7 days | Optional |
| **Enterprise** | Multi-cluster annual | Offline JWT + optional heartbeat | 14 days | Optional |
| **NFR** | Partners | Offline JWT only | 7 days | Not required |

**Trial is offline**: Trial licenses are 30-day offline JWTs generated automatically by the license portal. Air-gapped customers can fully evaluate the product without internet access.

### License JWT Payload

```json
{
  "jti": "uuid",
  "sub": "customer-uuid",
  "iss": "license.lattice.run",
  "iat": 1746000000,
  "exp": 1777536000,
  "customer_name": "Acme Corp",
  "type": "enterprise",
  "features": ["oidc", "turn", "telemetry", "audit", "ai-audit", "multi-cluster", "webhook", "siem"],
  "limits": {
    "max_nodes": 500,
    "max_clusters": 10
  },
  "public_key_id": "pk-2026-01"
}
```

### Public Key Versioning and Rotation

The binary embeds a list of trusted public keys indexed by `public_key_id`. License validation uses the public key matching the `public_key_id` in the JWT header.

```go
var trustedPublicKeys = map[string]ed25519.PublicKey{
    "pk-2026-01": { /* embedded bytes */ },
    // New keys added here on rotation; old keys retained to validate existing licenses
}
```

**Rotation process**:
1. Generate new Ed25519 keypair, assign next ID (e.g., `pk-2026-02`)
2. Add new public key to binary in next release; old key remains for existing licenses
3. New licenses issued with new key ID from that date forward
4. Old key deprecated after all licenses signed with it have expired (typically 1-2 years)
5. In case of private key compromise: emergency binary release removing compromised key ID; affected customers get new licenses

### License File Loading (Multi-path)

The server resolves the license file in priority order:

1. `$LATTICE_LICENSE_PATH` (environment variable — preferred for containers/K8s secret injection)
2. `$LATTICE_LICENSE` (environment variable containing the JWT string directly)
3. `~/.lattice/license.lic` (user home, for development)
4. `/var/lib/lattice/license.lic` (system default, Linux production)

### Validation Flow

1. **Service Startup**:
   - Resolve license file via multi-path loading above
   - No license found on Pro binary → startup failure: `"No valid license found. Install with: lattice license install ./license.lic"`
   - Parse and verify JWT signature using `public_key_id` → key not found or invalid signature → startup failure
   - Check `exp` claim (expiration)
     - Not expired → continue
     - Expired → enter grace period (type-dependent)
   - Load feature toggles from `features[]` and `type`
   - Store `last_valid_at` timestamp to disk (encrypted, see Clock Protection below)

2. **Online Heartbeat** (optional, Standard/Enterprise):
   - Configured via `lattice.yaml`: `license.heartbeat.enabled: true` (default: `false` for air-gapped compatibility)
   - Endpoint configurable: `license.heartbeat.url` (default: `https://license.lattice.run/api/v1/heartbeat`)
   - Every 24 hours: `POST` with `{ license_id, instance_id, version, node_count, ts }`
   - Server response includes authoritative `server_time`; client stores this as `last_known_server_time`
   - `200 OK` → normal
   - `403 Revoked` → disable Pro features immediately
   - Connection failure → grace period +1 day (max cumulative: Standard 7 days, Enterprise 14 days)
   - Beyond grace period → Pro features degraded, core networking unaffected

3. **Clock Manipulation Protection**:
   - On each successful heartbeat, store `last_known_server_time` to an encrypted local file
   - On startup, if `system_time < last_known_server_time` → clock rollback detected → treat as expired
   - For non-heartbeat deployments (air-gapped): rely on offline expiry only; clock protection is best-effort

4. **API Access Control**:
   - Every Pro feature API request intercepted by `middleware/license_check.go`
   - Check license status + feature toggle
   - Feature not licensed → `402 Payment Required` with body `{"error": "feature_not_licensed", "feature": "oidc"}`
   - Node limit check: cached count updated on registration/deletion, checked against `limits.max_nodes`

### Error Handling

| Scenario | Behavior | User Visible |
|----------|----------|-------------|
| No license on Pro start | Startup failure | CLI: "No valid license found" |
| Unknown public key ID | Startup failure | CLI: "License signed with unknown key" |
| Invalid JWT signature | Startup failure | CLI: "License verification failed" |
| License expired (within grace) | Pro available, warning | Dashboard yellow banner: "Expires in X days" |
| License expired (past grace) | Pro degraded, core unaffected | Dashboard red banner, API returns 402 |
| Heartbeat consecutive failure | Grace countdown | Dashboard: "Cannot reach license server" |
| License revoked | Immediately disable Pro | Dashboard: "License revoked" |
| Node limit exceeded | Reject new node registration | API: 429 "License limit exceeded" |
| Clock rollback detected | Treat as expired | Dashboard: "System clock anomaly detected" |

## Air-Gapped Deployment

Enterprise customers in regulated environments (finance, government, defense) that cannot access the public internet are fully supported:

- Trial: Offline JWT, no network required
- Standard/Enterprise: Offline JWT validation only; heartbeat disabled by default
- License portal generates offline licenses; customer downloads `.lic` file and installs manually
- No heartbeat = no revocation signal for air-gapped deployments; mitigation is annual renewal requirement (license `exp` enforces this)
- License path loaded from `$LATTICE_LICENSE_PATH` or K8s secret mount for containerized deployments

## License Server (Your Side)

```
license.lattice.run
├── Portal: customer self-service purchase, license download, renewal (see Phase 4)
├── Generate license: sign Ed25519 JWT with current private key
├── Admin dashboard: customer list, activation status, usage stats
├── Revocation API: mark license_id as revoked (heartbeat clients pick this up)
└── Heartbeat API: receive heartbeat, return { status, server_time }
```

## Implementation Phases

| Phase | Content | Deliverable |
|-------|---------|-------------|
| **Phase 1: License Core** | Ed25519 JWT verification, multi-path loading, license CLI (`install`/`show`/`validate`) | Installable and verifiable licenses |
| **Phase 2: Enterprise Feature Isolation** | Complete existing community stubs, unify 402 error handling, API middleware | Community build contains no enterprise code |
| **Phase 3: Online Heartbeat** | Optional heartbeat service, grace period logic, clock protection, degradation strategy | License lifecycle management |
| **Phase 4a: License Portal Backend** | Stripe/LemonSqueezy integration, license generation API, customer account system, subscription lifecycle | Customers can purchase and receive licenses automatically |
| **Phase 4b: License Portal Frontend** | Self-service portal UI, license download, renewal, subscription management | Customers self-manage licenses end-to-end |
| **Phase 5: Enterprise Feature Completion** | AI audit, multi-cluster management, Webhook/SIEM integration | Complete enterprise edition |

**Note on Phase 4**: Phase 4 is a full SaaS backend system (payment processing, subscription lifecycle, customer accounts), not just a UI panel. It is split into 4a (backend) and 4b (frontend) and should be scoped as an independent project with its own design document.

## Test Strategy

| Test Type | Content |
|-----------|---------|
| **Unit** | JWT signature verification, expiry check, grace period calculation, clock rollback detection, multi-path loading |
| **Integration** | Community build verification (Pro code excluded via build tags), Pro build verification |
| **E2E** | License installation flow, expiry degradation flow, optional heartbeat flow, air-gapped flow |
| **Security** | License forgery attacks, JWT tampering, clock manipulation, replay attack protection, key rotation |

## Industry Reference

Similar OpenCore implementations for reference:

| Product | Domain | Pattern | Key Learnings |
|---------|--------|---------|--------------|
| Teleport | Network access | Go build tags + offline JWT trial | Air-gapped first-class, JWT format |
| Grafana | Monitoring | Go build tags + JWT license | Public binary + license file model |
| HashiCorp Vault | Secret management | `ent` build tag | Heartbeat must be optional (enterprise firewall) |
| GitLab | DevOps | Runtime feature flags | Single binary simpler to maintain but pro code visible |
