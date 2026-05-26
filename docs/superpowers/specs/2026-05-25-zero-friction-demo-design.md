# Zero-Friction Demo Design

**Date:** 2026-05-25
**Status:** Approved

## Overview

Allow any visitor to launch a temporary Lattice demo in one click — no account, no manual token creation, no configuration. The visitor runs one curl command on each of two devices; both devices auto-enroll into an isolated demo workspace and connect over an encrypted WireGuard mesh.

## Goals

- Time-to-first-handshake under 3 minutes for a first-time user
- Zero backend resources pre-provisioned (no dummy peers, no hosted demo server)
- Self-hosted safe: disabled by default, no risk to production workloads
- Abuse-resistant: rate-limited by IP when enabled

## Non-Goals

- Hosted demo environment on alattice.io (user runs this on their own latticed)
- Single-device demo (requires two real devices to show connectivity)
- Persistent demo workspaces (TTL-based expiry only)

---

## Architecture

### Backend

#### Config

Three new config fields (environment variable / values.yaml / latticed config file):

```yaml
demo:
  enabled: false          # Default off — must be explicitly enabled
  ttlMinutes: 60          # Demo workspace lifetime (default 60 min)
  rateLimitPerHour: 5     # Max demo launches per IP per hour (in-memory counter)
```

#### Data Model

`Workspace` table gains two columns:

```go
IsDemo    bool       `gorm:"default:false"`
ExpiresAt *time.Time `gorm:"index"`
```

No changes to `EnrollmentToken` — it already has `expires_at` and a usage `limit`.

#### New Endpoint

```
POST /api/v1/demo/launch
```

- No `Authorization` header required
- Guarded by `demo.enabled` flag (returns `403` when disabled)
- IP rate-limited (`demo.rateLimitPerHour`); returns `429` when exceeded

**Request body:** empty

**Response (200):**

```json
{
  "workspace_id": "demo-abc123",
  "expires_at": "2026-05-25T08:00:00Z",
  "device1_cmd": "curl -fsSL https://<host>/install.sh | bash -s -- --server https://<host> --token lt-enroll-xxx",
  "device2_cmd": "curl -fsSL https://<host>/install.sh | bash -s -- --server https://<host> --token lt-enroll-xxx"
}
```

Both `device1_cmd` and `device2_cmd` use the **same enrollment token** created with `limit=2`. Each device consumes one usage.

The `<host>` is derived from the incoming request's `Host` header (or `X-Forwarded-Host` behind a reverse proxy).

**Internal steps on each launch:**

1. Check `demo.enabled`; return `403` if false
2. Check IP rate limit; return `429` if exceeded
3. Create workspace: `name=demo-<nanoid>`, `is_demo=true`, `expires_at=now+ttlMinutes`
4. Create default-allow policy scoped to the demo workspace
5. Create enrollment token: `limit=2`, `expires_at=expires_at`
6. Build and return the two curl commands

#### Cleanup Goroutine

Started at latticed boot. Runs every 5 minutes.

```
SELECT * FROM workspaces WHERE is_demo=true AND expires_at < NOW()
→ for each: delete peers, policies, tokens, workspace (cascade)
```

Uses a single DB transaction per workspace. Logs each deletion at `INFO` level.

---

### install.sh Changes

Add CLI argument parsing alongside existing environment variable support.

New flags:

| Flag | Equivalent env var | Purpose |
|------|--------------------|---------|
| `--server <url>` | `SERVER` | Control plane URL for `lattice init` |
| `--token <token>` | `TOKEN` | Enrollment token for `lattice init` |
| `--binary <name>` | `BINARY` | Binary to install (lattice / latticed) |
| `--tag <version>` | `TAG` | Specific version to install |

When `--server` and `--token` are both provided, the script appends two steps after binary installation:

```bash
lattice init --server "$SERVER" --token "$TOKEN" --non-interactive
lattice up --detach
```

This makes the demo curl command fully non-interactive: run it, agent enrolls, mesh is live.

**Backward compatibility:** all existing environment variable usage continues to work unchanged.

---

### Frontend

#### Entry Point

Landing page (`pages/index.vue`) CTA section: add a **"Try Demo"** secondary button alongside the existing primary CTA. Button is only rendered when the backend returns `demo.enabled=true` (checked via a lightweight `GET /api/v1/demo/status` endpoint, or embedded in the initial page config).

#### localStorage Persistence

Key: `lattice_demo` (JSON object)

```json
{
  "workspace_id": "demo-abc123",
  "expires_at": "2026-05-25T08:00:00Z",
  "device1_cmd": "...",
  "device2_cmd": "..."
}
```

On "Try Demo" click:
1. Read `localStorage["lattice_demo"]`
2. If exists and `expires_at > now` → open modal with cached data (no API call)
3. If missing or expired → call `POST /api/v1/demo/launch` → store response → open modal

#### Modal States

```
idle → loading → ready
              ↘ error (rate limit, disabled, network)
ready → expired (when countdown reaches zero)
```

#### Modal Layout

```
┌─────────────────────────────────────────┐
│  Zero-Friction Demo                     │
│  Expires in: 47:23                      │
├─────────────────────────────────────────┤
│  Step 1 — Run on Device 1               │
│  ┌─────────────────────────────────┐   │
│  │ curl -fsSL ... | bash -s -- ... │ copy │
│  └─────────────────────────────────┘   │
│                                         │
│  Step 2 — Run on Device 2               │
│  ┌─────────────────────────────────┐   │
│  │ curl -fsSL ... | bash -s -- ... │ copy │
│  └─────────────────────────────────┘   │
│                                         │
│  Step 3 — Verify (on either device)     │
│  lattice status                         │
│  ping <peer-ip>                         │
├─────────────────────────────────────────┤
│  [Start New Demo]           [Close]     │
└─────────────────────────────────────────┘
```

- Countdown timer updates every second
- Each code block has a copy-to-clipboard button
- "Start New Demo" clears localStorage and calls API again
- On expiry: replace code blocks with "This demo has expired" + "Start New Demo" button

---

## Error Handling

| Scenario | Backend response | Frontend behavior |
|----------|-----------------|-------------------|
| Demo disabled | `403 demo disabled` | Hide "Try Demo" button entirely |
| Rate limit exceeded | `429 Too Many Requests` | Show "Too many demo sessions from your network. Try again in X minutes." |
| Token already used (limit=2 consumed) | n/a (handled by enrollment flow) | Not surfaced in modal |
| Workspace expired mid-session | n/a (timer handles it) | Modal switches to expired state |

---

## Security

- Endpoint disabled by default; operator must opt in
- IP rate limiting prevents workspace flooding
- Demo workspaces are fully isolated (separate namespace/workspace)
- Demo peers cannot reach non-demo peers (default-allow policy scoped to demo workspace only)
- TTL-based auto-cleanup prevents orphaned resources
- No sensitive data in localStorage (token is enrollment-only, single-use per device)

---

## Files Affected

| File | Change |
|------|--------|
| `internal/db/models/workspace.go` | Add `IsDemo`, `ExpiresAt` fields |
| `internal/server/handler/demo.go` | New — `DemoHandler`, `LaunchDemo` |
| `internal/server/router.go` | Register `POST /api/v1/demo/launch` |
| `internal/server/demo_cleanup.go` | New — cleanup goroutine |
| `cmd/latticed/main.go` | Start cleanup goroutine on boot |
| `docs/public/install.sh` | Add `--server`, `--token`, `--binary`, `--tag` arg parsing |
| `frontend/src/pages/index.vue` | Add "Try Demo" button |
| `frontend/src/components/DemoModal.vue` | New — modal component |
| `deploy/charts/lattice/values.yaml` | Add `demo.*` config keys |
| `config/lattice/` | Add demo config to all-in-one overlay |
