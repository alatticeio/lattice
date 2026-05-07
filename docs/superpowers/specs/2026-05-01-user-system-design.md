# User System Design

> Status: Approved | 2026-05-01

## Motivation

Lattice serves both individual users and enterprise teams. A robust user system is foundational for:

- **Multi-tenancy**: Isolated workspaces with independent networks, peers, policies
- **Team collaboration**: Role-based access control for ops teams, developers, auditors
- **Security**: JWT with revocation, invitation-only onboarding, audit trail
- **Enterprise readiness**: OIDC/SSO integration via Dex for corporate identity providers

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                     Frontend (Vue 3)                  │
│  Pinia useUserStore  │  axios interceptor            │
│  (token in localStorage)  │  X-Workspace-Id header   │
└──────────────────────┬───────────────────────────────┘
                       │ REST JSON + JWT Bearer
┌──────────────────────▼───────────────────────────────┐
│                  Gin HTTP Router                      │
│  CORS → Audit → AuthMiddleware → Permission          │
└──────────────────────┬───────────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────────┐
│              Controller Layer                         │
│  userCtrl  workspaceCtrl  memberCtrl  invitationCtrl  │
└──────────────────────┬───────────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────────┐
│               Service Layer                           │
│  userService  workspaceService  invitationService     │
│              permission.Checker                       │
└──────────────────────┬───────────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────────┐
│          Store / Repository (GORM)                    │
│  Users │ Workspaces │ Members │ Invitations          │
│  Identities │ AuditLog │ Profiles                    │
└──────────────────────────────────────────────────────┘
```

---

## 1. Authentication

### 1.1 Registration

```
POST /api/v1/users/register
Body: { username, password }
```

- Password hashed with bcrypt
- User created with `SystemRole: "user"`
- On success, redirect to login

### 1.2 Login

```
POST /api/v1/users/login
Body: { username, password, client? }
```

- Lookup user by username
- bcrypt password comparison
- Generate JWT signed with HMAC-SHA256:
  - Web client: **12 hours** expiration
  - CLI client (`client: "cli"`): **30 days** expiration
- Response: `{ user, token }`

### 1.3 JWT Claims

```go
type LatticeClaims struct {
    jwt.RegisteredClaims  // sub=userID, iss="lattice-bff"
    Email      string
    Username   string
    SystemRole string     // "platform_admin" | "user"
}
```

**Design decision**: Workspace ID is intentionally NOT embedded in the JWT. Instead, it's passed per-request via the `X-Workspace-Id` header. This allows a single JWT to access multiple workspaces without re-authentication.

JWT secret: `LATTICE_JWT_SECRET` environment variable.

### 1.4 Logout & Token Revocation

```
POST /api/v1/auth/logout  (requires auth)
```

- Adds `jti` (JWT ID) to in-memory `RevocationList` with expiry
- Background goroutine purges expired entries every 5 minutes
- Frontend: removes token from `localStorage`, redirects to `/auth/login`

### 1.5 Auth Middleware

Every protected endpoint runs through `AuthMiddleware`:
1. Extract `Bearer <token>` from `Authorization` header
2. Parse & validate JWT
3. Check revocation list
4. Inject into context: `user_id`, `username`, `email`, `system_role`

### 1.6 Admin Bootstrap

At server startup, `InitAdmin()` creates or promotes users from `config.GlobalConfig.App.InitAdmins` to `SystemRole: "platform_admin"`.

---

## 2. Two-Tier Permission Model

### 2.1 System Roles (Platform Level)

| Role | Scope | Permissions |
|------|-------|------------|
| `platform_admin` | Entire platform | Bypass all workspace checks, manage all workspaces, platform settings |
| `user` | Per-workspace | Access only via workspace membership |

### 2.2 Workspace Roles

| Role | Weight | Permissions |
|------|--------|------------|
| `admin` | 100 | CRUD resources + manage members + manage invitations |
| `editor` | 80 | CRUD resources, cannot manage members |
| `member` | 40 | View resources + limited operations |
| `viewer` | 10 | Read-only access |

Role weights enable comparison: user with weight X can perform actions requiring weight ≤ X.

### 2.3 WorkspaceAuthMiddleware

Enforces workspace-scoped access:
1. Parse JWT → inject user info
2. Resolve workspace ID from `X-Workspace-Id` header
3. **Platform admin bypass**: `system_role == "platform_admin"` skips all checks
4. Check membership via `permission.Checker.RequireWorkspaceRole()`
5. Inject workspace context: `workspace_id`, `currentTeamMember`

**Convenience wrappers:**
- `AdminOnly()` → requires workspace `admin` role
- `PlatformAdminOnly()` → requires `platform_admin` system role

### 2.4 Route Permission Map

| Route | Middleware | Minimum Role |
|-------|-----------|-------------|
| `/users/register`, `/users/login` | None | Public |
| `/users/getme`, `/users/list`, `/profile/*` | `AuthMiddleware` | Authenticated |
| `/users/:id/system-role` | `AuthMiddleware` + inline | `platform_admin` |
| `/workspaces/add`, `/workspaces/list` | `AuthMiddleware` | Authenticated |
| `/workspaces/:id` (PUT/DELETE) | `AdminOnly` | Workspace `admin` |
| `/workspaces/:id/members` | `AdminOnly` | Workspace `admin` |
| `/workspaces/:id/invitations` | `AdminOnly` | Workspace `admin` |
| `/invite/:token` | None | Public |
| `/invite/:token/register` | None | Public |
| `/invite/:token/accept` | `AuthMiddleware` | Authenticated |
| `/networks/*`, `/peers/*`, `/policies/*` | `WorkspaceAuthMiddleware(viewer)` | `viewer`+ |
| `/settings/platform` | `PlatformAdminOnly` | `platform_admin` |

### 2.5 K8s Impersonation

For CRD operations, the `permission.Checker.K8sClient()` returns an impersonated K8s client:
- Impersonates user `wf-user-<userID>`
- In group `wf-group-<wsID>-<role>`
- Maps to RoleBindings created during workspace initialization

---

## 3. Workspace & Membership

### 3.1 Workspace Lifecycle

```
POST /api/v1/workspaces/add     Create workspace
PUT  /api/v1/workspaces/:id     Update workspace
DELETE /api/v1/workspaces/:id   Delete workspace (cascades)
GET  /api/v1/workspaces/list    List accessible workspaces
```

**Creation flow:**
1. Validate slug uniqueness (per-user, not globally — two users can have a "test" space)
2. Create `Workspace` DB record
3. Set K8s namespace `wf-<workspaceID>` with labels
4. Create `ResourceQuota` (max 50 nodes, 20 secrets)
5. Create `RoleBinding`s for all 4 workspace roles
6. Create default `LatticeNetwork` (CIDR `100.64.0.0/16`)
7. Create default `LatticePolicy` (action `DENY`)
8. Add creator as workspace `admin`

### 3.2 Membership

```
GET    /api/v1/workspaces/:id/members            List members
POST   /api/v1/workspaces/:id/members/:userID    Add member
PUT    /api/v1/workspaces/:id/members/:userID    Update role
DELETE /api/v1/workspaces/:id/members/:userID    Remove member
```

- **Privilege escalation prevention**: Cannot assign a role higher than your own
- **Soft removal**: Sets `status = "removed"` rather than deleting the record
- **Self-removal**: Allowed; removing others requires admin

### 3.3 Visibility

- Platform admins: see all workspaces
- Regular users: see only workspaces where they are active members
- Workspace list enriched with: node count, quota usage, network name/CIDR/status

---

## 4. Invitation System

### 4.1 Invitation Model

```
POST   /api/v1/workspaces/:id/invitations        Create invitation
GET    /api/v1/workspaces/:id/invitations        List invitations
DELETE /api/v1/workspaces/:id/invitations/:id    Revoke invitation

GET    /api/v1/invite/:token                     Preview (public)
POST   /api/v1/invite/:token/register            Register + accept (public)
POST   /api/v1/invite/:token/accept              Accept for logged-in user
```

### 4.2 Create Flow

1. Only workspace admins can invite
2. Cannot invite with a role higher than the inviter's own
3. Prevent duplicate pending invitations per email+workspace
4. Generate HMAC-SHA256 signed token: `hex(random(16)).hex(HMAC(random, secret))`
5. **7-day expiration**

### 4.3 Accept Flow (Existing User)

1. Validate token signature
2. Check invitation is pending and not expired
3. Verify logged-in user's email matches invitation email
4. Create `WorkspaceMember` with `status: "active"`
5. Mark invitation `accepted`

### 4.4 Register + Accept Flow (New User)

1. Same validation as Accept
2. Create user account
3. Create workspace membership
4. Return signed JWT (immediately logged in)

### 4.5 Revocation

- Platform admins can revoke any invitation
- Inviter or workspace admins can revoke their own
- Sets status to `"revoked"` (soft delete)

---

## 5. Third-Party Auth (Dex/OIDC — Pro)

### 5.1 Architecture

```
User → Dex IdP (external OIDC) → /auth/callback → Lattice JWT
```

- Behind `//go:build pro` build tag
- Community stub returns `402 Payment Required`
- Activated when `config.GlobalConfig.Dex.ProviderUrl` is configured

### 5.2 OIDC Flow

1. User redirected to Dex for authentication
2. Dex redirects to `GET /auth/callback?code=<code>`
3. Server exchanges code for OIDC tokens
4. Parse & verify `id_token` with Dex public keys
5. Call `userService.OnboardExternalUser()`:
   - Check email against `AdminEmails` whitelist for `platform_admin` role
   - Look up existing identity by provider + subject
   - Create user if not found
   - Create `UserIdentity` record linking external IdP to platform user
6. Issue JWT, redirect to frontend: `http://localhost:5173/login/success?token=<jwt>`

### 5.3 Identity Model

`UserIdentity` links a platform user to an external identity provider:
- Supports: `local`, `dex`, `ldap`, `github`
- Tracks `ExternalID`, `Email`, `Metadata` (JSON), `LastSyncAt`

---

## 6. Audit System

Applied to all non-GET, non-OPTIONS requests via `AuditMiddleware`:

| Field | Source |
|-------|--------|
| `UserID`, `UserName`, `UserEmail` | JWT claims |
| `UserIP` | Request |
| `WorkspaceID` | `X-Workspace-Id` header |
| `Action` | HTTP method + path mapping |
| `Resource` | URL extraction |
| `Status` | Response status code |

Special-cased actions: `LOGIN`, `LOGOUT`, `ACCEPT`, `INVITE`, `REVOKE`.

---

## 7. Data Model Summary

### Tables

| Table | Purpose |
|-------|---------|
| `t_user` | User accounts with bcrypt passwords |
| `t_user_profile` | Optional profile info (company, bio, timezone) |
| `t_user_identity` | External IdP links (Dex, LDAP, GitHub) |
| `t_workspace` | Workspace metadata and status |
| `t_workspaces_member` | Role bindings: user ↔ workspace + role |
| `t_workspace_invitation` | Pending invitations with HMAC tokens |
| `t_audit_log` | Immutable audit trail |

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Workspace ID in header, not JWT | Single token, multi-workspace |
| Role weights (numeric comparison) | Simple, no policy engine needed |
| Soft removal (status field) | Audit trail, undo support |
| HMAC invitation tokens | Stateless verification, no DB lookup needed |
| Platform admin bypass | Emergency access, debugging |
| K8s impersonation for CRD ops | Leverages K8s native RBAC, no custom admission webhooks |
| In-memory revocation list | Fast lookup, acceptable trade-off (logout requires re-bounce) |

## File Layout

```
internal/server/models/user.go               User, UserProfile models
internal/server/models/identity.go           UserIdentity model
internal/server/models/token.go              LatticeClaims JWT type
internal/server/models/workspace.go          Workspace, WorkspaceMember, Invitation models
internal/server/server/user.go               User HTTP handlers
internal/server/server/workspace.go          Workspace HTTP handlers
internal/server/server/member.go             Member HTTP handlers
internal/server/server/invitation.go         Invitation HTTP handlers
internal/server/server/api.go                Route registration + OIDC setup
internal/server/server/server.go             Server bootstrap + InitAdmin
internal/server/server/middleware/auth.go    JWT verification + revocation
internal/server/server/middleware/permission.go   Workspace RBAC middleware
internal/server/server/middleware/audit.go   Audit logging middleware
internal/server/service/user.go             User + onboarding logic
internal/server/service/workspace.go        Workspace lifecycle + K8s init
internal/server/service/invitation.go       Invitation create/accept/revoke
internal/server/permission/checker.go       Role-based permission checker
internal/server/resource/impersonator.go    K8s identity impersonation
internal/server/dto/workspace.go            SystemRole, WorkspaceRole types
internal/server/repository/base.go          Generic CRUD base
internal/server/auth/revocation.go          In-memory token revocation
internal/db/gormstore/user.go              User GORM repository
internal/db/gormstore/workspace.go         Workspace + member GORM repository
internal/db/gormstore/identity.go          UserIdentity GORM repository
internal/db/gormstore/migrate.go           Auto-migration
internal/agent/store/store.go             Store interface (all repos)
pkg/utils/jwt.go                           JWT generate + parse
pkg/utils/encrypt.go                       bcrypt wrapper
fronted/src/api/user.ts                    Frontend user API client
fronted/src/api/invitation.ts              Frontend invitation API client
fronted/src/api/request.ts                 Axios interceptor (token + workspace)
fronted/src/stores/user.ts                 Pinia user auth store
fronted/src/router/guard/authGuard.ts      Vue Router auth guard
fronted/src/utils/auth.ts                  Token localStorage helpers
```
