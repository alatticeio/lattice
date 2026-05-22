# Agent Isolation Wiring Design

Date: 2026-05-09

## Context

The `agent-isolation` feature has been partially implemented across several commits. The service layer (`AgentIsolationService`, `AgentRegistrationService`), middleware (`AgentAuth`), models, and DB migrations are all complete. What remains is wiring these into the HTTP server.

## Goal

Enable the full agent enrollment and isolation flow end-to-end:

1. Admin creates a one-time enrollment token via API
2. Agent registers with the token and receives a JWT
3. Agent authenticates via JWT to call `/api/v1/agents/tools/call`
4. `AgentIsolationService` enforces tool-call policies on those calls
5. Admin can revoke agents

## Out of Scope

- Frontend UI for agent management
- Changes to the existing `agentRouter` / `AgentEnrollService`

## Architecture

Two changes, minimal surface area:

### 1. `internal/server/server/server.go`

**Struct fields (append to Server):**
```go
agentIsolationService service.AgentIsolationService
agentRegService       service.AgentRegistrationService
```

**Initialization in `NewServer()` (after AI service init):**

```
if cfg.AI.AgentIsolation.Enabled:
    jwtSecret := cfg.AI.AgentIsolation.JWTSecret
    if jwtSecret == "":
        jwtSecret = cfg.JWT.Secret
    agentIsolationService = service.NewAgentIsolationService(cfg.AI.AgentIsolation)
    agentRegService = service.NewAgentRegistrationService(jwtSecret, st, client.GetClient())
```

Pass `agentIsolationService` to both `NewAIServiceWithWorkflow` calls (replacing the existing `nil` arguments at position 8).

**Route registration in `apiRouter()`:**
```go
s.agentIsolationRouter()
```

### 2. `internal/server/server/agent_isolation_router.go` (new file)

Base path: `/api/v1/agent-isolation`
Auth: `AuthMiddleware` on all routes
Guard: if `agentRegService == nil`, return `402 Payment Required`

| Method | Path | Handler | Purpose |
|--------|------|---------|---------|
| POST | `/enrollment-tokens` | `handleCreateEnrollmentToken` | Admin creates one-time token |
| POST | `/register` | `handleAgentRegister` | Agent exchanges token for JWT |
| DELETE | `/agents/:name` | `handleAgentRevoke` | Admin revokes agent |

**Request/Response contracts:**

`POST /enrollment-tokens`
```json
// request
{ "namespace": "default", "allowedTools": ["list_peers"], "ttlSeconds": 3600 }
// response
{ "token": "abc123...", "expiresAt": "2026-05-09T10:00:00Z" }
```

`POST /register`
```json
// request
{ "enrollmentToken": "abc123...", "agentName": "claude-agent-1", "publicKey": "wg-pubkey..." }
// response
{ "jwt": "eyJ...", "agentIdentityName": "claude-agent-1" }
```

`DELETE /agents/:name?namespace=<ns>`
```json
// response
{ "data": null }
```

## Configuration

To enable the feature, set in `lattice.yaml`:

```yaml
ai:
  agent-isolation:
    enabled: true
    enforcement-mode: enforce   # disabled | audit | enforce
    audit-level: write          # none | write | full
    jwt-secret: "<32+ char secret>"
```

If `jwt-secret` is empty, falls back to `jwt.secret`.

## Error Handling

- `agentRegService == nil` (feature disabled): `402 Payment Required`
- Invalid/missing fields: `400 Bad Request`
- Enrollment token not found / expired / already used: `400 Bad Request` (opaque message to avoid enumeration)
- K8s create failure: `500 Internal Server Error`

## Testing

Existing unit tests cover service logic. No new service tests needed. The router handlers are thin and follow existing patterns — no new handler tests added (consistent with project convention for simple CRUD handlers).
