# Sandbox Try Demo Implementation Design

## Goal

Add a "Try Sandbox" demo flow to the landing page that lets users spin up a sandbox enrollment token, copy a one-liner `lattice sandbox run` command with a preset agent (claude / python3 / curl), and open the web console to see the agent online and its traffic audit log.

## Architecture

The sandbox demo is completely independent from the AI Network demo. It has its own backend endpoint, its own workspace lifecycle, and its own frontend modal. The two demos share no state.

**Tech Stack:** Go/Gin backend, Vue 3 + Vite + Tailwind 4 frontend, existing demo infrastructure (workspace controller, token controller, policy controller, magic session map, sweep cleanup goroutine).

---

## File Map

### New files
- `frontend/src/components/SandboxDemoModal.vue` — modal UI (preset selector, run command, timer, console link)

### Modified files
- `internal/server/server/demo.go` — add `sandboxLaunchResponse`, `handleSandboxDemoLaunch()`, route registration
- `frontend/src/pages/auth/demo.vue` — support `?redirect=` query param
- `frontend/src/pages/index.vue` — add "Try Sandbox" button + `SandboxDemoModal`

---

## Backend

### New endpoint: `POST /api/v1/demo/sandbox/launch`

Registered alongside the existing demo routes in `demoRouter()`.

**Request:** no body (same as network launch).

**Steps:**
1. Check `cfg.Enabled` — return 403 if demo is disabled.
2. Create workspace: slug `sandbox-demo-<unixmilli>`, display name `Sandbox Demo Workspace`, `IsDemo=true`, `ExpiresAt = now + TTL`.
3. Create enrollment token: `limit=1` (one sandbox agent), expiry = TTL string.
4. Apply allow-all policy (identical logic to network demo, network name `lattice-default-net`).
5. Create demo user (`demo-<rand12>`) + login + add to workspace as admin.
6. Issue magic token → store in `demoSessions` map with `expiresAt`.
7. Build `console_url = <serverURL>/auth/demo?token=<magic>&redirect=/sandbox`.
8. Build `install_cmd` (same curl install.sh pattern as network demo, version-tagged when clean release).
9. Return `sandboxLaunchResponse`.

**Response type:**
```go
type sandboxLaunchResponse struct {
    WorkspaceID string    `json:"workspace_id"`
    ExpiresAt   time.Time `json:"expires_at"`
    ServerURL   string    `json:"server_url"`
    Token       string    `json:"token"`      // enrollment token
    InstallCmd  string    `json:"install_cmd"`
    ConsoleURL  string    `json:"console_url"`
}
```

**Cleanup:** No changes needed. `sweepExpiredDemos` already queries all workspaces where `is_demo=true AND expires_at < now`, so sandbox demo workspaces are automatically cleaned up.

**Rate limiting:** Reuses the existing `demoLimiter` middleware with the same per-hour limit.

---

## Frontend

### `frontend/src/pages/auth/demo.vue`

Add `?redirect=` param support. After successful auth, if `redirect` param is present and starts with `/`, navigate there instead of `/dashboard`.

```ts
const redirect = params.get('redirect')
const target = (redirect && redirect.startsWith('/')) ? redirect : '/dashboard'
router.replace(target)
```

### `frontend/src/components/SandboxDemoModal.vue`

Mirrors `DemoModal.vue` structure and state machine (`loading | ready | expired | error`).

**localStorage key:** `lattice_demo_sandbox`

**Session interface:**
```ts
interface SandboxDemoSession {
  workspace_id: string
  expires_at: string
  server_url: string
  token: string
  install_cmd: string
  console_url: string
}
```

**Preset commands** (computed from `session.server_url` + `session.token`):

| Key | Command suffix |
|-----|---------------|
| `claude` | `-- claude --model claude-opus-4-6` |
| `python3` | `-- python3 -c "import urllib.request; print(urllib.request.urlopen('https://httpbin.org/get').read().decode())"` |
| `curl` | `-- curl -s https://httpbin.org/get` |

Full run command:
```
lattice sandbox run --name demo-agent --server-url <server_url> --token <token> -- <preset-suffix>
```

**Steps shown in modal:**
1. **Install** — shows `install_cmd` with copy button. Note: "requires Pro binary on Linux"
2. **Run agent** — preset dropdown (claude / python3 / curl) + full `sandbox run` command with copy button
3. **Open Console** — button → `window.open(session.console_url, '_blank')`

**Timer badge:** Same logic as `DemoModal.vue` (green → amber → red, pulse dot, `expired` state on zero).

**New session / reset:** `localStorage.removeItem('lattice_demo_sandbox')` + re-call launch.

### `frontend/src/pages/index.vue`

Add `sandboxDemoOpen` ref and `SandboxDemoModal` component alongside the existing `demoOpen` / `DemoModal`. Add a "Try Sandbox" button in the hero or features section, next to "Try Demo".

---

## Data Flow

```
User clicks "Try Sandbox"
  → SandboxDemoModal opens, calls POST /api/v1/demo/sandbox/launch
  → Backend creates workspace, token, user, magic session
  → Frontend stores session in localStorage, shows install_cmd + run_cmd
  → User runs install on Linux Pro machine, then runs sandbox run command
  → Agent registers with enrollment token, appears in /sandbox page
  → User clicks "Open Console" → /auth/demo?token=...&redirect=/sandbox
  → demo.vue exchanges magic token for JWT, sets active_ws, navigates to /sandbox
  → User sees agent online and traffic audit log
```

---

## Error Handling

- Demo disabled → 403, modal shows error state with message.
- Rate limit exceeded → 429, modal shows "Too many sandbox demo sessions. Please try again later."
- `install_cmd` or `run_cmd` copy on HTTP → `execCopy` fallback (already in codebase from DemoModal.vue).
- Magic token expired before auth → `handleDemoAuth` returns 401, `demo.vue` shows error + back link.

---

## Out of Scope

- Sandbox demo does not auto-run any agent server-side; the user must run the command locally.
- No changes to sandbox enrollment token schema or agent-isolation backend.
- No changes to the existing AI Network demo flow.
