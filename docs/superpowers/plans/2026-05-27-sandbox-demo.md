# Sandbox Try Demo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an independent "Try Sandbox" demo flow — a new modal on the landing page that provisions a sandbox workspace + enrollment token, shows a `lattice sandbox run` one-liner with preset agent commands (claude / python3 / curl), and links to the sandbox console.

**Architecture:** New `POST /api/v1/demo/sandbox/launch` endpoint mirrors the existing `handleDemoLaunch` pattern (workspace → token → policy → demo user → magic token). Frontend adds `SandboxDemoModal.vue` with a preset button group and a timer. `/auth/demo.vue` gains `?redirect=` support so the console URL lands on `/sandbox`.

**Tech Stack:** Go 1.25 / Gin, Vue 3.5 + Vite + Tailwind 4, existing `demoSessions` sync.Map, existing workspace/token/policy/user controllers.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/server/server/demo.go` | Modify | Add `sandboxLaunchResponse`, `handleSandboxDemoLaunch()`, register route |
| `frontend/src/pages/auth/demo.vue` | Modify | Support `?redirect=` param for post-auth navigation |
| `frontend/src/components/SandboxDemoModal.vue` | Create | Sandbox demo modal with preset selector, timer, copy, console link |
| `frontend/src/pages/index.vue` | Modify | Import SandboxDemoModal, add `sandboxDemoOpen` ref, add "Try Sandbox" button |

---

## Task 1: Backend — sandbox demo endpoint

**Files:**
- Modify: `internal/server/server/demo.go`

### Context

The existing `handleDemoLaunch` in `demo.go` is the pattern to follow. Key helpers already in scope:
- `s.demoCfg()` — returns TTL/rate limit config
- `s.workspaceController.AddWorkspace`, `s.store.Workspaces().GetByID`, `Update`
- `s.tokenController.Create(tokenCtx, &dto.TokenDto{...})`
- `s.policyController.ApplyDirect(tokenCtx, wsID, "", "", &dto.PolicyDto{...})`
- `s.userController.Register`, `Login`; `s.memberController.Add`
- `utils.GenerateRandomBytes`, `base64.RawURLEncoding.EncodeToString`
- `s.demoSessions.Store(magicToken, demoMagicSession{...})`
- `isCleanRelease(version.Version)` and `version.Version`
- `resp.OK`, `resp.Error`

- [ ] **Step 1: Add `sandboxLaunchResponse` struct and route registration**

In `internal/server/server/demo.go`, add the response type and register the new route inside `demoRouter()`:

```go
// sandboxLaunchResponse is returned by POST /api/v1/demo/sandbox/launch.
type sandboxLaunchResponse struct {
	WorkspaceID string    `json:"workspace_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	ServerURL   string    `json:"server_url"`
	Token       string    `json:"token"`
	InstallCmd  string    `json:"install_cmd"`
	ConsoleURL  string    `json:"console_url"`
}
```

Update `demoRouter()` to add:

```go
func (s *Server) demoRouter() {
	s.GET("/api/v1/demo/status", s.handleDemoStatus())
	s.GET("/api/v1/demo/auth", s.handleDemoAuth())
	s.POST("/api/v1/demo/launch",
		s.demoLimiter.Middleware(rate.Limit(s.demoCfg().RateLimitPerHour)/3600, 1),
		s.handleDemoLaunch(),
	)
	s.POST("/api/v1/demo/sandbox/launch",
		s.demoLimiter.Middleware(rate.Limit(s.demoCfg().RateLimitPerHour)/3600, 1),
		s.handleSandboxDemoLaunch(),
	)
}
```

- [ ] **Step 2: Implement `handleSandboxDemoLaunch`**

Add the full handler in `internal/server/server/demo.go`:

```go
func (s *Server) handleSandboxDemoLaunch() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := s.demoCfg()
		if !cfg.Enabled {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "demo is disabled"})
			return
		}

		ctx := c.Request.Context()
		ttl := time.Duration(cfg.TTLMinutes) * time.Minute
		expiresAt := time.Now().Add(ttl)

		// 1. Create workspace
		slug := fmt.Sprintf("sandbox-demo-%d", time.Now().UnixMilli())
		wsVo, err := s.workspaceController.AddWorkspace(ctx, &dto.WorkspaceDto{
			Slug:        slug,
			DisplayName: "Sandbox Demo Workspace",
		})
		if err != nil {
			resp.Error(c, "failed to create sandbox demo workspace: "+err.Error())
			return
		}
		rollback := func() { _ = s.workspaceController.DeleteWorkspace(context.Background(), wsVo.ID) }

		// 2. Mark workspace as demo with expiry
		ws, err := s.store.Workspaces().GetByID(ctx, wsVo.ID)
		if err != nil {
			rollback()
			resp.Error(c, "failed to fetch workspace: "+err.Error())
			return
		}
		ws.IsDemo = true
		ws.ExpiresAt = &expiresAt
		if err = s.store.Workspaces().Update(ctx, ws); err != nil {
			rollback()
			resp.Error(c, "failed to mark sandbox demo workspace: "+err.Error())
			return
		}

		// 3. Create enrollment token (limit=1, one sandbox agent)
		tokenCtx := context.WithValue(ctx, infra.WorkspaceKey, wsVo.ID)
		expiry := fmt.Sprintf("%dm", cfg.TTLMinutes)
		tokenStr, err := s.tokenController.Create(tokenCtx, &dto.TokenDto{
			Namespace: wsVo.Namespace,
			Limit:     1,
			Expiry:    expiry,
		})
		if err != nil {
			rollback()
			resp.Error(c, "failed to create enrollment token: "+err.Error())
			return
		}

		// 4. Apply allow-all policy
		const demoNetwork = "lattice-default-net"
		networkLabel := fmt.Sprintf("alattice.io/network-%s", demoNetwork)
		peerSel := metav1.LabelSelector{
			MatchLabels: map[string]string{networkLabel: "true"},
		}
		if _, policyErr := s.policyController.ApplyDirect(tokenCtx, wsVo.ID, "", "", &dto.PolicyDto{
			Name:        "demo-allow-all",
			Action:      "Allow",
			PolicyTypes: []string{"Ingress", "Egress"},
			LatticePolicySpec: v1alpha1.LatticePolicySpec{
				Network:      demoNetwork,
				PeerSelector: peerSel,
				Action:       "ALLOW",
				Ingress: []v1alpha1.IngressRule{
					{From: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
				},
				Egress: []v1alpha1.EgressRule{
					{To: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
				},
			},
		}); policyErr != nil {
			s.logger.Warn("sandbox demo: failed to apply allow-all policy (non-fatal)", "err", policyErr)
		}

		// 5. Create demo user
		rawBytes := make([]byte, 16)
		if err = utils.GenerateRandomBytes(rawBytes); err != nil {
			rollback()
			resp.Error(c, "failed to generate demo credentials: "+err.Error())
			return
		}
		randSuffix := base64.RawURLEncoding.EncodeToString(rawBytes)[:12]
		demoUsername := fmt.Sprintf("demo-%s", randSuffix)
		demoPassword := base64.RawURLEncoding.EncodeToString(rawBytes)

		if err = s.userController.Register(ctx, dto.UserDto{
			Username: demoUsername,
			Password: demoPassword,
		}); err != nil {
			rollback()
			resp.Error(c, "failed to create demo user: "+err.Error())
			return
		}

		demoUser, err := s.userController.Login(ctx, demoUsername, demoPassword)
		if err != nil {
			rollback()
			resp.Error(c, "failed to authenticate demo user: "+err.Error())
			return
		}

		if err = s.memberController.Add(ctx, wsVo.ID, demoUser.ID, dto.RoleAdmin); err != nil {
			rollback()
			resp.Error(c, "failed to add demo user to workspace: "+err.Error())
			return
		}

		// 6. Issue magic token
		magicRaw := make([]byte, 32)
		if err = utils.GenerateRandomBytes(magicRaw); err != nil {
			rollback()
			resp.Error(c, "failed to generate magic token: "+err.Error())
			return
		}
		magicToken := base64.RawURLEncoding.EncodeToString(magicRaw)
		s.demoSessions.Store(magicToken, demoMagicSession{
			userID:               demoUser.ID,
			workspaceID:          wsVo.ID,
			workspaceNamespace:   wsVo.Namespace,
			workspaceSlug:        wsVo.Slug,
			workspaceDisplayName: wsVo.DisplayName,
			expiresAt:            expiresAt,
		})

		// 7. Build URLs
		scheme := "https"
		if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		host := c.Request.Host
		if fwdHost := c.GetHeader("X-Forwarded-Host"); fwdHost != "" {
			host = fwdHost
		}
		serverURL := fmt.Sprintf("%s://%s", scheme, host)
		installURL := fmt.Sprintf("%s/install.sh", serverURL)

		var installCmd string
		if isCleanRelease(version.Version) {
			installCmd = fmt.Sprintf(
				"curl -fsSL %s | bash -s -- --server %s --token %s --tag %s",
				installURL, serverURL, tokenStr, version.Version,
			)
		} else {
			installCmd = fmt.Sprintf(
				"curl -fsSL %s | bash -s -- --server %s --token %s",
				installURL, serverURL, tokenStr,
			)
		}

		consoleURL := fmt.Sprintf("%s/auth/demo?token=%s&redirect=/sandbox", serverURL, magicToken)

		resp.OK(c, sandboxLaunchResponse{
			WorkspaceID: wsVo.ID,
			ExpiresAt:   expiresAt,
			ServerURL:   serverURL,
			Token:       tokenStr,
			InstallCmd:  installCmd,
			ConsoleURL:  consoleURL,
		})
	}
}
```

- [ ] **Step 3: Build and lint**

```bash
make lint
```

Expected: `0 issues.`

- [ ] **Step 4: Commit**

```bash
git add internal/server/server/demo.go
git commit -s -m "feat(demo): add sandbox demo launch endpoint"
```

---

## Task 2: Frontend — auth/demo.vue redirect support

**Files:**
- Modify: `frontend/src/pages/auth/demo.vue`

### Context

Current file (`frontend/src/pages/auth/demo.vue`) always calls `router.replace('/dashboard')` after successful auth. We need to read an optional `?redirect=` query param and use it if present and starts with `/`.

- [ ] **Step 1: Add redirect param support**

Replace the `router.replace('/dashboard')` line with:

```ts
const redirect = params.get('redirect')
const target = (redirect && redirect.startsWith('/')) ? redirect : '/dashboard'
router.replace(target)
```

Full updated `onMounted` block:

```ts
onMounted(async () => {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')

  if (!token) {
    error.value = 'Missing demo token.'
    return
  }

  try {
    const res = await fetch(`/api/v1/demo/auth?token=${encodeURIComponent(token)}`)
    const body = await res.json()

    if (!res.ok || body.code !== 200) {
      error.value = body.message ?? 'Demo session is invalid or has expired.'
      return
    }

    setToken(body.data.token)
    if (body.data.refreshToken) {
      setRefreshToken(body.data.refreshToken)
    }

    if (body.data.workspace) {
      const ws = body.data.workspace
      localStorage.setItem('active_ws', JSON.stringify(ws))
      localStorage.setItem('active_ws_id', ws.id)
    }

    const redirect = params.get('redirect')
    const target = (redirect && redirect.startsWith('/')) ? redirect : '/dashboard'
    router.replace(target)
  } catch {
    error.value = 'Network error. Please try again.'
  }
})
```

- [ ] **Step 2: Build to verify TypeScript**

```bash
make build-ui 2>&1 | tail -5
```

Expected: `>>> UI built → internal/web/dist`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/auth/demo.vue
git commit -s -m "feat(demo): support ?redirect= param in /auth/demo for post-auth navigation"
```

---

## Task 3: Frontend — SandboxDemoModal.vue

**Files:**
- Create: `frontend/src/components/SandboxDemoModal.vue`

### Context

Mirrors `DemoModal.vue` patterns: same state machine (`loading | ready | expired | error`), same timer countdown, same `execCopy` fallback, same localStorage cache. No `Select` component exists — use a segmented button group for presets (same pattern as the scheme toggle in `frontend/src/pages/settings/platform/index.vue`). Icons available: `Container`, `Loader2`, `RefreshCw`, `Copy`, `Check`, `ExternalLink` from `lucide-vue-next`.

- [ ] **Step 1: Create the component**

Create `frontend/src/components/SandboxDemoModal.vue`:

```vue
<script setup lang="ts">
import { ref, computed, onUnmounted, watch } from 'vue'
import { Copy, Check, RefreshCw, ExternalLink, Container, Loader2 } from 'lucide-vue-next'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

interface SandboxDemoSession {
  workspace_id: string
  expires_at: string
  server_url: string
  token: string
  install_cmd: string
  console_url: string
}

const STORAGE_KEY = 'lattice_demo_sandbox'
const openModel = defineModel<boolean>('open')

type State = 'loading' | 'ready' | 'expired' | 'error'
type Preset = 'claude' | 'python3' | 'curl'

const state = ref<State>('loading')
const session = ref<SandboxDemoSession | null>(null)
const errorMsg = ref('')
const timeLeft = ref('')
const remainingMs = ref(0)
const copiedInstall = ref(false)
const copiedRun = ref(false)
const preset = ref<Preset>('claude')

let timer: ReturnType<typeof setInterval> | null = null

const timerClass = computed(() => {
  if (remainingMs.value <= 60_000) return 'text-red-500'
  if (remainingMs.value <= 300_000) return 'text-amber-500'
  return 'text-emerald-500'
})

function formatTime(ms: number): string {
  if (ms <= 0) return '0:00'
  const totalSec = Math.floor(ms / 1000)
  const m = Math.floor(totalSec / 60)
  const s = totalSec % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

function startCountdown(expiresAt: string) {
  if (timer) clearInterval(timer)
  remainingMs.value = Math.max(0, new Date(expiresAt).getTime() - Date.now())
  timeLeft.value = formatTime(remainingMs.value)
  timer = setInterval(() => {
    const ms = new Date(expiresAt).getTime() - Date.now()
    remainingMs.value = Math.max(0, ms)
    if (ms <= 0) {
      timeLeft.value = '0:00'
      state.value = 'expired'
      clearInterval(timer!)
    } else {
      timeLeft.value = formatTime(ms)
    }
  }, 1000)
}

const presets: { value: Preset; label: string; suffix: string }[] = [
  {
    value: 'claude',
    label: 'claude',
    suffix: '-- claude --model claude-opus-4-6',
  },
  {
    value: 'python3',
    label: 'python3',
    suffix: "-- python3 -c \"import urllib.request; print(urllib.request.urlopen('https://httpbin.org/get').read().decode())\"",
  },
  {
    value: 'curl',
    label: 'curl',
    suffix: '-- curl -s https://httpbin.org/get',
  },
]

const runCmd = computed(() => {
  if (!session.value) return ''
  const p = presets.find(x => x.value === preset.value)!
  return `lattice sandbox run --name demo-agent --server-url ${session.value.server_url} --token ${session.value.token} ${p.suffix}`
})

async function launch() {
  state.value = 'loading'
  errorMsg.value = ''
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const cached: SandboxDemoSession = JSON.parse(raw)
      if (new Date(cached.expires_at).getTime() > Date.now()) {
        session.value = cached
        state.value = 'ready'
        startCountdown(cached.expires_at)
        return
      }
    }
  } catch { /* ignore */ }

  try {
    const res = await fetch('/api/v1/demo/sandbox/launch', { method: 'POST' })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      errorMsg.value = (body as { message?: string }).message
        ?? (res.status === 429 ? 'Too many sandbox demo sessions. Please try again later.' : 'Failed to launch sandbox demo.')
      state.value = 'error'
      return
    }
    const data: SandboxDemoSession = (await res.json()).data
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
    session.value = data
    state.value = 'ready'
    startCountdown(data.expires_at)
  } catch {
    errorMsg.value = 'Network error. Please check your connection.'
    state.value = 'error'
  }
}

function reset() {
  localStorage.removeItem(STORAGE_KEY)
  session.value = null
  launch()
}

function execCopy(text: string) {
  const el = document.createElement('textarea')
  el.value = text
  el.setAttribute('readonly', '')
  el.style.cssText = 'position:fixed;top:0;left:0;width:2em;height:2em;opacity:0;pointer-events:none'
  document.body.appendChild(el)
  el.focus()
  el.select()
  try { document.execCommand('copy') } catch { /* ignore */ }
  document.body.removeChild(el)
}

async function copy(text: string, which: 'install' | 'run') {
  if (navigator.clipboard) {
    try { await navigator.clipboard.writeText(text) } catch { execCopy(text) }
  } else {
    execCopy(text)
  }
  if (which === 'install') {
    copiedInstall.value = true
    setTimeout(() => { copiedInstall.value = false }, 2000)
  } else {
    copiedRun.value = true
    setTimeout(() => { copiedRun.value = false }, 2000)
  }
}

function openConsole() {
  if (session.value?.console_url) window.open(session.value.console_url, '_blank')
}

watch(openModel, (v) => { if (v) launch() })
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <Dialog v-model:open="openModel">
    <DialogContent class="max-w-xl p-0 overflow-hidden gap-0">

      <!-- Header -->
      <div class="px-6 pt-6 pb-5 border-b border-border">
        <div class="flex items-start justify-between">
          <div class="flex items-center gap-2.5">
            <div class="flex items-center justify-center w-8 h-8 rounded-lg bg-primary/10">
              <Container class="size-4 text-primary" />
            </div>
            <div>
              <h2 class="text-base font-semibold leading-none">Try Sandbox</h2>
              <p class="text-xs text-muted-foreground mt-1">Run an AI agent in an isolated network sandbox</p>
            </div>
          </div>
          <div v-if="state === 'ready'" class="flex items-center gap-1.5 rounded-full border border-border px-2.5 py-1 text-xs font-mono font-medium" :class="timerClass">
            <span class="size-1.5 rounded-full bg-current animate-pulse" />
            {{ timeLeft }}
          </div>
        </div>
      </div>

      <!-- Body -->
      <div class="px-6 py-5">

        <!-- Loading -->
        <div v-if="state === 'loading'" class="flex flex-col items-center gap-3 py-10 text-muted-foreground">
          <Loader2 class="size-5 animate-spin" />
          <span class="text-sm">Setting up your sandbox workspace…</span>
        </div>

        <!-- Error -->
        <div v-else-if="state === 'error'" class="space-y-4 py-2">
          <div class="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3">
            <p class="text-sm text-destructive">{{ errorMsg }}</p>
          </div>
          <Button variant="outline" size="sm" @click="launch" class="gap-1.5">
            <RefreshCw class="size-3.5" /> Try Again
          </Button>
        </div>

        <!-- Expired -->
        <div v-else-if="state === 'expired'" class="space-y-4 py-2">
          <div class="rounded-lg border border-border bg-muted/40 px-4 py-3">
            <p class="text-sm text-muted-foreground">This sandbox session has expired.</p>
          </div>
          <Button variant="outline" size="sm" @click="reset" class="gap-1.5">
            <RefreshCw class="size-3.5" /> Start New Session
          </Button>
        </div>

        <!-- Ready -->
        <div v-else-if="state === 'ready' && session" class="space-y-4">

          <!-- Step 1: Install -->
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <span class="flex items-center justify-center size-5 rounded-full bg-primary text-primary-foreground text-[11px] font-bold shrink-0">1</span>
              <span class="text-sm font-medium">Install on Linux <span class="text-xs text-muted-foreground font-normal">(Pro binary required)</span></span>
            </div>
            <div class="group relative rounded-lg bg-zinc-950 dark:bg-zinc-900 border border-zinc-800 px-4 py-3 pr-12">
              <code class="text-xs text-zinc-100 font-mono break-all whitespace-pre-wrap leading-relaxed">{{ session.install_cmd }}</code>
              <button
                class="absolute top-2.5 right-2.5 flex items-center justify-center size-7 rounded-md transition-colors"
                :class="copiedInstall ? 'bg-emerald-500/20 text-emerald-400' : 'bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-100'"
                @click="copy(session!.install_cmd, 'install')"
              >
                <Check v-if="copiedInstall" class="size-3.5" />
                <Copy v-else class="size-3.5" />
              </button>
            </div>
          </div>

          <!-- Step 2: Run agent -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span class="flex items-center justify-center size-5 rounded-full bg-primary text-primary-foreground text-[11px] font-bold shrink-0">2</span>
                <span class="text-sm font-medium">Run your agent</span>
              </div>
              <!-- Preset toggle (segmented button group) -->
              <div class="flex rounded-md border border-input overflow-hidden text-xs font-mono">
                <button
                  v-for="p in presets"
                  :key="p.value"
                  type="button"
                  :class="preset === p.value ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground hover:bg-muted/80'"
                  class="px-2 py-0.5 border-l border-input first:border-l-0 transition-colors"
                  @click="preset = p.value"
                >{{ p.label }}</button>
              </div>
            </div>
            <div class="group relative rounded-lg bg-zinc-950 dark:bg-zinc-900 border border-zinc-800 px-4 py-3 pr-12">
              <code class="text-xs text-zinc-100 font-mono break-all whitespace-pre-wrap leading-relaxed">{{ runCmd }}</code>
              <button
                class="absolute top-2.5 right-2.5 flex items-center justify-center size-7 rounded-md transition-colors"
                :class="copiedRun ? 'bg-emerald-500/20 text-emerald-400' : 'bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-100'"
                @click="copy(runCmd, 'run')"
              >
                <Check v-if="copiedRun" class="size-3.5" />
                <Copy v-else class="size-3.5" />
              </button>
            </div>
          </div>

          <!-- Step 3: Open Console -->
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <span class="flex items-center justify-center size-5 rounded-full bg-muted text-muted-foreground text-[11px] font-bold shrink-0">3</span>
              <span class="text-sm font-medium">View agent in console</span>
            </div>
            <div class="rounded-lg bg-zinc-950 dark:bg-zinc-900 border border-zinc-800 px-4 py-3">
              <div class="flex items-center gap-2 font-mono text-xs">
                <span class="text-zinc-500 select-none">#</span>
                <span class="text-zinc-400">Agent appears in /sandbox once the command runs</span>
              </div>
            </div>
          </div>

          <!-- Footer actions -->
          <div class="flex items-center justify-between pt-1">
            <button class="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors" @click="reset">
              <RefreshCw class="size-3" /> New Session
            </button>
            <Button
              v-if="session.console_url"
              size="sm"
              class="gap-1.5 h-8 text-xs"
              @click="openConsole"
            >
              <ExternalLink class="size-3.5" /> Open Console
            </Button>
          </div>
        </div>

      </div>
    </DialogContent>
  </Dialog>
</template>
```

- [ ] **Step 2: Build to verify TypeScript**

```bash
make build-ui 2>&1 | tail -5
```

Expected: `>>> UI built → internal/web/dist`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/SandboxDemoModal.vue
git commit -s -m "feat(demo): add SandboxDemoModal component"
```

---

## Task 4: Frontend — Landing page "Try Sandbox" button

**Files:**
- Modify: `frontend/src/pages/index.vue`

### Context

Current state of the relevant section in `index.vue`:
- Line 24: `import DemoModal from '@/components/DemoModal.vue'`
- Line 48: `const demoOpen = ref(false)`
- Lines 298-306: "Try Demo" button (shown only when `demoEnabled`)
- Line 604: `<DemoModal v-model:open="demoOpen" />`

- [ ] **Step 1: Add import and ref**

In `index.vue`, update the import section (near line 24) to add:

```ts
import SandboxDemoModal from '@/components/SandboxDemoModal.vue'
```

Add below `const demoOpen = ref(false)` (near line 48):

```ts
const sandboxDemoOpen = ref(false)
```

- [ ] **Step 2: Add "Try Sandbox" button next to "Try Demo"**

The current button block (lines 298-306) is:

```html
<Button
  v-if="demoEnabled"
  variant="outline"
  size="lg"
  class="gap-2 px-7"
  @click="demoOpen = true"
>
  Try Demo
</Button>
```

Replace with:

```html
<Button
  v-if="demoEnabled"
  variant="outline"
  size="lg"
  class="gap-2 px-7"
  @click="demoOpen = true"
>
  Try Demo
</Button>
<Button
  v-if="demoEnabled"
  variant="outline"
  size="lg"
  class="gap-2 px-7"
  @click="sandboxDemoOpen = true"
>
  Try Sandbox
</Button>
```

- [ ] **Step 3: Add SandboxDemoModal to template**

The current last line of the template (line 604) is:

```html
  <DemoModal v-model:open="demoOpen" />
```

Replace with:

```html
  <DemoModal v-model:open="demoOpen" />
  <SandboxDemoModal v-model:open="sandboxDemoOpen" />
```

- [ ] **Step 4: Build to verify TypeScript**

```bash
make build-ui 2>&1 | tail -5
```

Expected: `>>> UI built → internal/web/dist`

- [ ] **Step 5: Run lint**

```bash
make lint
```

Expected: `0 issues.`

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/index.vue
git commit -s -m "feat(demo): add Try Sandbox button and SandboxDemoModal to landing page"
```

---

## Manual Verification

After all tasks complete:

1. Start `latticed` with demo enabled (`app.demo.enabled: true` in config).
2. Open landing page — both "Try Demo" and "Try Sandbox" buttons should be visible.
3. Click "Try Sandbox" → modal opens, spins up workspace, shows install_cmd and run_cmd for `claude` preset.
4. Switch preset to `python3` / `curl` — run command updates without reload.
5. Copy run command — button turns green checkmark.
6. On a Linux machine with Pro binary: run the install command, then run the sandbox run command.
7. Click "Open Console" → browser opens `/auth/demo?token=...&redirect=/sandbox`, logs in, lands on `/sandbox` page, agent appears as online.
8. Wait for TTL — modal shows red timer, then transitions to "expired" state.
9. Wait 5 minutes after expiry — `sweepExpiredDemos` deletes the workspace automatically.
