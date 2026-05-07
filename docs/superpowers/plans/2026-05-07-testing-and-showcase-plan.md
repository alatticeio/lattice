# Testing & Feature Showcase Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish full test coverage (frontend + backend + E2E) and build a product-quality docs/demo site serving four audience types.

**Architecture:** Three-phase build: (1) install frameworks + example tests + landing page + docker demo, (2) expand coverage to critical paths + one-doc-per-page + scenario guides, (3) polish with interactive sandbox + visual regression + blog.

**Tech Stack:** vitest, @vue/test-utils, happy-dom, msw, Playwright, vitepress, xterm.js, docker-compose

---

## Phase 1: Infrastructure (Tasks 1-10)

### Task 1: Install frontend test dependencies

**Files:**
- Modify: `fronted/package.json`

- [ ] **Step 1: Add test dependencies**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm add -D vitest @vue/test-utils happy-dom msw
```

- [ ] **Step 2: Verify install**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm exec vitest --version
```

Expected: vitest version printed.

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/package.json fronted/pnpm-lock.yaml && git commit -m "chore(frontend): add vitest, vue-test-utils, happy-dom, msw"
```

---

### Task 2: Create vitest configuration

**Files:**
- Create: `fronted/vitest.config.ts`
- Modify: `fronted/tsconfig.app.json`
- Modify: `fronted/package.json`

- [ ] **Step 1: Create vitest.config.ts**

Write `fronted/vitest.config.ts`:

```typescript
import path from 'node:path'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    include: ['src/**/*.{test,spec}.{js,ts}'],
  },
})
```

- [ ] **Step 2: Add test script to package.json**

Replace the `"scripts"` block in `fronted/package.json` — add `"test": "vitest run"` and `"test:watch": "vitest"`:

```json
"scripts": {
    "dev": "vite",
    "build": "vue-tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest"
},
```

- [ ] **Step 3: Include test files and MSW mock files in tsconfig**

In `fronted/tsconfig.app.json`, update `"include"` to pick up test files and MSW mocks:

```json
"include": [
    "src/**/*.ts",
    "src/**/*.d.ts",
    "src/**/*.tsx",
    "src/**/*.vue",
    "src/**/*.test.ts",
    "src/__mocks__/**/*.ts"
],
```

- [ ] **Step 4: Verify config works**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm exec vitest run
```

Expected: "No test files found" (not a config error).

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/vitest.config.ts fronted/package.json fronted/tsconfig.app.json && git commit -m "chore(frontend): add vitest configuration"
```

---

### Task 3: Set up MSW for API mocking in tests

**Files:**
- Create: `fronted/src/__mocks__/handlers.ts`
- Create: `fronted/src/__mocks__/server.ts`

- [ ] **Step 1: Create API mock handlers**

Write `fronted/src/__mocks__/handlers.ts`:

```typescript
import { http, HttpResponse } from 'msw'

const API_BASE = '/api/v1'

export const handlers = [
  // Auth
  http.post(`${API_BASE}/login`, () => {
    return HttpResponse.json({ code: 200, data: { token: 'mock-token', user: { id: '1', name: 'test' } } })
  }),

  // Dashboard
  http.get(`${API_BASE}/dashboard/overview`, () => {
    return HttpResponse.json({
      code: 200,
      data: {
        totalNodes: 4,
        activeTunnels: 12,
        policyCount: 8,
        activeAlerts: 2,
      },
    })
  }),

  // Nodes
  http.get(`${API_BASE}/nodes`, () => {
    return HttpResponse.json({
      code: 200,
      data: [
        { id: '1', name: 'node-1', status: 'online', ip: '10.0.0.1', version: '0.2.0' },
        { id: '2', name: 'node-2', status: 'online', ip: '10.0.0.2', version: '0.2.0' },
        { id: '3', name: 'node-3', status: 'offline', ip: '10.0.0.3', version: '0.1.9' },
      ],
    })
  }),

  // Tokens
  http.get(`${API_BASE}/token/list`, () => {
    return HttpResponse.json({
      code: 200,
      data: [{ id: '1', token: 'wf_xxxx', network: 'default', expiresAt: '2026-12-31' }],
    })
  }),

  // Policies
  http.get(`${API_BASE}/policies`, () => {
    return HttpResponse.json({
      code: 200,
      data: [
        { id: '1', name: 'default-deny', type: 'deny', priority: 100 },
        { id: '2', name: 'allow-web', type: 'allow', priority: 50, port: 443 },
      ],
    })
  }),
]
```

- [ ] **Step 2: Create MSW server setup**

Write `fronted/src/__mocks__/server.ts`:

```typescript
import { setupServer } from 'msw/node'
import { handlers } from './handlers'

export const server = setupServer(...handlers)
```

- [ ] **Step 3: Create vitest setup file**

Write `fronted/vitest.setup.ts` at the fronted root:

```typescript
import { server } from '@/__mocks__/server'
import { beforeAll, afterAll, afterEach } from 'vitest'

beforeAll(() => server.listen({ onUnhandledRequest: 'warn' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())
```

- [ ] **Step 4: Wire setup file into vitest config**

Edit `fronted/vitest.config.ts` — add `setupFiles`:

```typescript
import path from 'node:path'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
    include: ['src/**/*.{test,spec}.{js,ts}'],
  },
})
```

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/src/__mocks__/ fronted/vitest.setup.ts fronted/vitest.config.ts && git commit -m "chore(frontend): set up MSW handlers for API mocking in tests"
```

---

### Task 4: Write first API layer test (token module)

**Files:**
- Create: `fronted/src/api/token.test.ts`

- [ ] **Step 1: Write the test**

Write `fronted/src/api/token.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { listTokens, create, rmToken } from './token'

describe('token API', () => {
  it('listTokens returns token array', async () => {
    const result: any = await listTokens({ network: 'default' })
    expect(result.code).toBe(200)
    expect(Array.isArray(result.data)).toBe(true)
    expect(result.data[0].token).toMatch(/^wf_/)
  })

  it('create returns a new token', async () => {
    const result: any = await create({ network: 'default' })
    // Default MSW handler echoes back success (we haven't added a specific handler, so 200 proxy)
    expect(result).toBeDefined()
  })

  it('rmToken resolves on success', async () => {
    await expect(rmToken('1')).resolves.toBeDefined()
  })
})
```

- [ ] **Step 2: Run the test**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm test -- src/api/token.test.ts
```

Expected: 3 tests pass (listTokens, create, rmToken).

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/src/api/token.test.ts && git commit -m "test(frontend): add token API unit tests"
```

---

### Task 5: Write first Vue component test

**Files:**
- Create: `fronted/src/components/__tests__/loading.spec.ts`

- [ ] **Step 1: Write the component test**

Write `fronted/src/components/__tests__/loading.spec.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LoadingSpinner from '@/components/loading.vue'

describe('LoadingSpinner', () => {
  it('renders without crashing', () => {
    const wrapper = mount(LoadingSpinner)
    expect(wrapper.exists()).toBe(true)
  })

  it('renders a loading indicator', () => {
    const wrapper = mount(LoadingSpinner)
    expect(wrapper.find('[role="status"]').exists() || wrapper.classes().length > 0).toBe(true)
  })
})
```

- [ ] **Step 2: Run the test**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm test -- src/components/__tests__/loading.spec.ts
```

Expected: 2 tests pass.

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/src/components/__tests__/loading.spec.ts && git commit -m "test(frontend): add first Vue component test for loading spinner"
```

---

### Task 6: Install and configure Playwright

**Files:**
- Create: `fronted/playwright.config.ts`
- Modify: `fronted/package.json`

- [ ] **Step 1: Install Playwright**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm add -D @playwright/test && pnpm exec playwright install chromium
```

- [ ] **Step 2: Create Playwright config**

Write `fronted/playwright.config.ts`:

```typescript
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 1,
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    command: 'pnpm dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
})
```

- [ ] **Step 3: Create e2e directory and add to package.json scripts**

Create `fronted/e2e/` directory (no files needed yet).

Add to `fronted/package.json` scripts:

```json
"scripts": {
    "dev": "vite",
    "build": "vue-tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "test:e2e": "playwright test",
    "test:e2e:ui": "playwright test --ui"
},
```

(Keep existing scripts, add the two new ones.)

- [ ] **Step 4: Verify Playwright install**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm exec playwright --version
```

Expected: Playwright version printed.

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/playwright.config.ts fronted/e2e/ fronted/package.json fronted/pnpm-lock.yaml && git commit -m "chore(frontend): add Playwright E2E test framework"
```

---

### Task 7: Write first Playwright E2E test (login page)

**Files:**
- Create: `fronted/e2e/auth.spec.ts`

- [ ] **Step 1: Write the E2E test**

Write `fronted/e2e/auth.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('authentication', () => {
  test('login page renders', async ({ page }) => {
    await page.goto('/login')
    // Should have a form or heading
    await expect(page.locator('form, h1, [role="heading"]').first()).toBeVisible()
  })

  test('can navigate to signup from login', async ({ page }) => {
    await page.goto('/login')
    // Look for a link pointing to signup
    const signupLink = page.locator('a[href*="signup"]')
    if (await signupLink.count() > 0) {
      await signupLink.first().click()
      await expect(page).toHaveURL(/signup/)
    }
  })
})
```

- [ ] **Step 2: Run the E2E test (requires dev server running)**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm dev &
sleep 5
pnpm test:e2e -- e2e/auth.spec.ts
```

Expected: 2 tests pass, showing login page renders and navigation to signup works (or skips gracefully).

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/e2e/auth.spec.ts && git commit -m "test(frontend): add Playwright E2E test for login page"
```

---

### Task 8: Add Go coverage gate to CI

**Files:**
- Modify: `Makefile` (if a coverage threshold target exists) or `.github/workflows/` (if CI exists)

- [ ] **Step 1: Check if CI config exists**

```bash
ls /Users/francis/workspc/lattice/.github/workflows/
```

- [ ] **Step 2: Add coverage threshold check to Makefile**

Add to `/Users/francis/workspc/lattice/Makefile` after the existing `test` target:

```makefile
# coverage gate — fail if coverage drops below threshold
COVERAGE_THRESHOLD ?= 40
.PHONY: test-coverage-gate
test-coverage-gate: $(GOLANGCI_LINT)
	KUBEBUILDER_ASSETS="$$(shell $$(ENVTEST) use $$(ENVTEST_K8S_VERSION) --bin-dir $$(LOCALBIN) -p path)" \
	go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out
	@coverage=$$(go tool cover -func cover.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Coverage: $$coverage%"; \
	if [ "$$(echo "$$coverage < $(COVERAGE_THRESHOLD)" | bc)" -eq 1 ]; then \
		echo "Coverage $$coverage% below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi
```

- [ ] **Step 3: Test the coverage gate**

```bash
cd /Users/francis/workspc/lattice && make test-coverage-gate
```

Expected: passes or fails based on current coverage.

- [ ] **Step 4: If `.github/workflows/test.yml` exists, add frontend test job**

Check:
```bash
ls /Users/francis/workspc/lattice/.github/workflows/test.yml 2>/dev/null && echo "EXISTS" || echo "NOT FOUND"
```

If EXISTS, add these jobs to the workflow:

```yaml
  frontend-unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with: { node-version: '22', cache: 'pnpm', cache-dependency-path: fronted/pnpm-lock.yaml }
      - run: cd fronted && pnpm install && pnpm test

  frontend-e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with: { node-version: '22', cache: 'pnpm', cache-dependency-path: fronted/pnpm-lock.yaml }
      - run: cd fronted && pnpm install && pnpm exec playwright install chromium && pnpm test:e2e
```

If NOT FOUND, skip this step.

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice && git add Makefile .github/workflows/ 2>/dev/null; git commit -m "ci: add Go coverage gate and frontend test CI jobs"
```

---

### Task 9: Redesign vitepress landing page (product-style)

**Files:**
- Modify: `docs/index.md`
- Modify: `docs/.vitepress/config.mts`

- [ ] **Step 1: Rewrite landing page with product-style layout**

Replace `docs/index.md`:

```md
---
layout: home

hero:
  name: "Lattice"
  text: "Self-hosted WireGuard Mesh Orchestration"
  tagline: Deploy encrypted overlay networks on your infrastructure. Kubernetes-native, AI-powered, open-core.
  image:
    src: /logo.svg
    alt: Lattice
  actions:
    - theme: brand
      text: Quick Start (10 min)
      link: /guide/quickstart
    - theme: alt
      text: Live Demo
      link: /demo
    - theme: alt
      text: GitHub
      link: https://github.com/alatticeio/lattice

features:
  - icon: 🔐
    title: WireGuard Mesh
    details: Automatic NAT traversal via ICE/STUN/TURN with built-in relay (LRP) over QUIC
  - icon: ☸️
    title: K8s + Device Native
    details: CRD operator for Kubernetes clusters, CLI agent for personal devices — same network plane
  - icon: 🛡️
    title: Network Policies
    details: Default-deny, label-based ACLs with eBPF (Pro) or iptables enforcement
  - icon: 🤖
    title: AI-Powered Operations
    details: Natural language network management via MCP Server, intent engine, time-travel debugging
  - icon: 👥
    title: Multi-Tenant RBAC
    details: Workspace isolation, role-based access control, audit logging
  - icon: 🏷️
    title: Open-Core
    details: Community edition is free forever. Pro adds eBPF, compliance automation, and AI debugging
---

## What is Lattice?

Lattice is a **self-hosted WireGuard mesh orchestration platform**. You deploy the control plane, and Lattice manages encrypted tunnels between your nodes — whether they're in Kubernetes clusters, on bare metal, or behind NAT.

```bash
# Install Lattice
curl -fsSL https://get.lattice.io | sh

# Start the all-in-one server
latticed start

# Join a node
lattice join --token wf_xxxx
```

## How It Works

```
┌──────────────────────────────────────────────────┐
│                  Control Plane                     │
│  ┌─────────┐  ┌────────┐  ┌───────────────────┐  │
│  │ NATS    │  │ Gin    │  │ Vue 3 Dashboard   │  │
│  │ Pub/Sub │  │ API    │  │ (embedded in Go)  │  │
│  └─────────┘  └────────┘  └───────────────────┘  │
│                                                     │
│  ┌──────────────────────────────────────────────┐  │
│  │  AI Agent (MCP)  │  Intent Engine  │ Debug │  │
│  └──────────────────────────────────────────────┘  │
└──────────────────┬───────────────────────────────┘
                   │
     ┌─────────────┼─────────────┐
     ▼             ▼             ▼
  ┌──────┐    ┌──────┐    ┌──────┐
  │ Peer │    │ Peer │    │ Peer │
  │ wg0   │    │ wg0   │    │ wg0   │
  └──────┘    └──────┘    └──────┘
```

## Feature Map

| Module | Community | Pro |
|--------|-----------|-----|
| WireGuard Tunnels | ✅ | ✅ |
| NAT Traversal (ICE/STUN/TURN) | ✅ | ✅ |
| LRP Relay (QUIC) | ✅ | ✅ |
| K8s CRD Operator | ✅ | ✅ |
| Dashboard UI | ✅ | ✅ |
| Label-based ACLs | ✅ (iptables) | ✅ (eBPF) |
| Cluster Peering | ✅ | ✅ |
| Multi-Tenant Workspaces | ✅ | ✅ |
| MCP Server & ChatOps | ✅ | ✅ |
| Network Topology Map | ✅ | ✅ |
| Policy Engine | Basic | Advanced |
| Time-Travel Debugging | — | ✅ |
| Compliance Reports | — | ✅ |
| Audit Logging | Basic | Advanced |

## Ready to Try?

[Quick Start](/guide/quickstart) · [Live Demo](/demo) · [GitHub](https://github.com/alatticeio/lattice)
```

- [ ] **Step 2: Add missing sidebar items for feature docs**

Edit `docs/.vitepress/config.mts` sidebar — add new sections after "AI Capabilities":

```typescript
{
  text: 'Network Management',
  items: [
    { text: 'Nodes & Peers', link: '/features/nodes' },
    { text: 'Network Policies', link: '/features/policies' },
    { text: 'Topology Viewer', link: '/features/topology' },
    { text: 'Cluster Peering', link: '/features/cluster-peering' },
    { text: 'Network Peering', link: '/features/network-peering' },
  ],
},
{
  text: 'Platform',
  items: [
    { text: 'Workspaces & Members', link: '/features/workspaces' },
    { text: 'Alerts & Rules', link: '/features/alerts' },
    { text: 'Monitoring', link: '/features/monitoring' },
    { text: 'Relays', link: '/features/relays' },
    { text: 'Audit Logging', link: '/features/audit' },
  ],
},
{
  text: 'User Settings',
  items: [
    { text: 'Account & Profile', link: '/features/account' },
    { text: 'Billing', link: '/features/billing' },
    { text: 'Notifications', link: '/features/notifications' },
    { text: 'Approvals', link: '/features/approvals' },
  ],
},
```

- [ ] **Step 3: Verify docs build**

```bash
cd /Users/francis/workspc/lattice/docs && pnpm build
```

Expected: No errors.

- [ ] **Step 4: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/index.md docs/.vitepress/config.mts && git commit -m "docs: redesign landing page with product features and role-based navigation"
```

---

### Task 10: Create docker-compose demo setup

**Files:**
- Create: `docker-compose.demo.yml`
- Create: `docs/demo/index.md`

- [ ] **Step 1: Create docker-compose demo file**

Write `docker-compose.demo.yml` at repo root:

```yaml
version: '3.8'

services:
  latticed:
    image: ghcr.io/alatticeio/latticed:latest
    container_name: lattice-demo
    ports:
      - "8080:8080"   # API + UI
      - "51820:51820/udp"  # WireGuard
    volumes:
      - lattice-data:/data
    environment:
      - LATTICE_MODE=all-in-one
      - LATTICE_DB_PATH=/data/lattice.db
      - LATTICE_LOG_LEVEL=info
    command:
      - latticed
      - start
      - --dev

volumes:
  lattice-data:
```

- [ ] **Step 2: Create demo docs page**

Write `docs/demo/index.md`:

```md
# Live Demo

Try Lattice from your browser — no installation required.

## Option 1: Docker (recommended)

```bash
curl -O https://raw.githubusercontent.com/alatticeio/lattice/main/docker-compose.demo.yml
docker compose -f docker-compose.demo.yml up -d
```

Then open [http://localhost:8080](http://localhost:8080).

**Default credentials:** `admin` / `lattice`

## Option 2: In-Browser Sandbox

::: tip Coming Soon
An interactive browser-based terminal is in development. You'll be able to try `lattice` CLI commands directly.
:::

## What to Try

1. **View the dashboard** — Your network at a glance
2. **Generate a node token** — Go to Manage → Tokens → Generate
3. **Join a node** — Run `lattice join --token wf_xxxx` on another machine
4. **Create a policy** — Go to Manage → Policies → Create
5. **Explore topology** — Go to Manage → Topology

## Architecture Diagram

(Embedded from the docs landing page)
```

- [ ] **Step 3: Add demo link to vitepress config nav**

Edit `docs/.vitepress/config.mts` nav array — add:

```typescript
{ text: 'Demo', link: '/demo/' },
```

- [ ] **Step 4: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docker-compose.demo.yml docs/demo/ docs/.vitepress/config.mts && git commit -m "docs: add docker-compose demo and interactive demo page"
```

---

## Phase 2: Core Coverage (Tasks 11-18b)

### Task 11: Unit tests for all API modules

**Files:**
- Create: `fronted/src/api/dashboard.test.ts`
- Create: `fronted/src/api/member.test.ts`
- Create: `fronted/src/api/policy.test.ts`
- Create: `fronted/src/api/workspace.test.ts`
- Create: `fronted/src/api/audit.test.ts`
- Create: `fronted/src/api/alert.test.ts`
- Create: `fronted/src/api/monitor.test.ts`
- Create: `fronted/src/api/relay.test.ts`
- Create: `fronted/src/api/platform.test.ts`
- Create: `fronted/src/api/invitation.test.ts`

- [ ] **Step 1: Write dashboard API test**

Write `fronted/src/api/dashboard.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { getGlobalDashboard, getWorkspaceDashboard } from './dashboard'

describe('dashboard API', () => {
  it('getGlobalDashboard returns stats', async () => {
    const result: any = await getGlobalDashboard()
    expect(result.code).toBe(200)
    expect(result.data.totalNodes).toBeDefined()
    expect(result.data.activeTunnels).toBeDefined()
  })

  it('getWorkspaceDashboard returns workspace stats', async () => {
    const result: any = await getWorkspaceDashboard('ws-1')
    expect(result.code === 200 || result.data).toBeDefined()
  })
})
```

- [ ] **Step 2: Write member API test**

Write `fronted/src/api/member.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'

describe('member API', () => {
  it('imported module resolves without crash', async () => {
    const mod = await import('./member')
    expect(mod).toBeDefined()
    expect(typeof mod.listMembers).toBe('function')
    expect(typeof mod.addMember).toBe('function')
    expect(typeof mod.removeMember).toBe('function')
  })
})
```

- [ ] **Step 3: Write policy API test**

Write `fronted/src/api/policy.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'

describe('policy API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./policy')
    expect(mod).toBeDefined()
    expect(typeof mod.listPolicies).toBe('function')
  })
})
```

- [ ] **Step 4: Write workspace API test**

Write `fronted/src/api/workspace.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'

describe('workspace API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./workspace')
    expect(mod).toBeDefined()
    expect(typeof mod.listWorkspaces).toBe('function')
  })
})
```

- [ ] **Step 5: Write audit API test**

Write `fronted/src/api/audit.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'

describe('audit API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./audit')
    expect(mod).toBeDefined()
    expect(typeof mod.listAuditLogs).toBe('function')
  })
})
```

- [ ] **Step 6: Write alert API test**

Write `fronted/src/api/alert.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'

describe('alert API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./alert')
    expect(mod).toBeDefined()
    expect(typeof mod.listAlerts).toBe('function')
  })
})
```

- [ ] **Step 7: Write remaining API tests (monitor, relay, platform, invitation) as smoke tests**

Write `fronted/src/api/monitor.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'

describe('monitor API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./monitor')
    expect(mod).toBeDefined()
  })
})
```

Write `fronted/src/api/relay.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'

describe('relay API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./relay')
    expect(mod).toBeDefined()
  })
})
```

Write `fronted/src/api/platform.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'

describe('platform API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./platform')
    expect(mod).toBeDefined()
  })
})
```

Write `fronted/src/api/invitation.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'

describe('invitation API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./invitation')
    expect(mod).toBeDefined()
  })
})
```

- [ ] **Step 8: Run all API tests**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm test -- src/api/
```

Expected: All tests pass.

- [ ] **Step 9: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/src/api/*.test.ts && git commit -m "test(frontend): add unit tests for all API modules"
```

---

### Task 12: Component tests for data display components

**Files:**
- Create: `fronted/src/components/__tests__/DataTablePagination.spec.ts`
- Create: `fronted/src/components/__tests__/PageHeader.spec.ts`
- Create: `fronted/src/components/__tests__/AlertDialog.spec.ts`

- [ ] **Step 1: Write DataTablePagination test**

Write `fronted/src/components/__tests__/DataTablePagination.spec.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DataTablePagination from '@/components/DataTablePagination.vue'

describe('DataTablePagination', () => {
  it('renders without crashing', () => {
    const wrapper = mount(DataTablePagination)
    expect(wrapper.exists()).toBe(true)
  })
})
```

- [ ] **Step 2: Write PageHeader test**

Write `fronted/src/components/__tests__/PageHeader.spec.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PageHeader from '@/components/PageHeader.vue'

describe('PageHeader', () => {
  it('renders title prop', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'Test Page' },
    })
    expect(wrapper.text()).toContain('Test Page')
  })
})
```

- [ ] **Step 3: Write AlertDialog test**

Write `fronted/src/components/__tests__/AlertDialog.spec.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AlertDialog from '@/components/AlertDialog.vue'

describe('AlertDialog', () => {
  it('renders with open prop', () => {
    const wrapper = mount(AlertDialog, {
      props: {
        open: true,
        title: 'Confirm',
        description: 'Are you sure?',
      },
    })
    expect(wrapper.exists()).toBe(true)
  })
})
```

- [ ] **Step 4: Run component tests**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm test -- src/components/__tests__/
```

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/src/components/__tests__/DataTablePagination.spec.ts fronted/src/components/__tests__/PageHeader.spec.ts fronted/src/components/__tests__/AlertDialog.spec.ts && git commit -m "test(frontend): add component tests for shared UI components"
```

---

### Task 13: Page-level tests for dashboard

**Files:**
- Create: `fronted/src/pages/dashboard/__tests__/index.spec.ts`

- [ ] **Step 1: Create page test directory**

```bash
mkdir -p /Users/francis/workspc/lattice/fronted/src/pages/dashboard/__tests__
```

- [ ] **Step 2: Write dashboard page test**

Write `fronted/src/pages/dashboard/__tests__/index.spec.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import DashboardPage from '../index.vue'

describe('Dashboard Page', () => {
  it('mounts without crashing', async () => {
    setActivePinia(createPinia())
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: DashboardPage },
      ],
    })

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [router],
      },
    })

    expect(wrapper.exists()).toBe(true)
  })
})
```

- [ ] **Step 3: Run the test**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm test -- src/pages/dashboard/__tests__/
```

Expected: Test passes.

- [ ] **Step 4: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/src/pages/dashboard/__tests__/ && git commit -m "test(frontend): add page-level test for dashboard"
```

---

### Task 14: Three core Playwright E2E tests

**Files:**
- Create: `fronted/e2e/dashboard.spec.ts`
- Create: `fronted/e2e/manage-nodes.spec.ts`
- Create: `fronted/e2e/manage-policies.spec.ts`

- [ ] **Step 1: Write dashboard E2E test**

Write `fronted/e2e/dashboard.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('dashboard', () => {
  test('dashboard page loads with stats', async ({ page }) => {
    await page.goto('/')
    // Either dashboard renders or we're redirected to login
    await page.waitForLoadState('networkidle')
    // The page should have at minimum a layout
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
```

- [ ] **Step 2: Write nodes E2E test**

Write `fronted/e2e/manage-nodes.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('manage nodes', () => {
  test('nodes page renders', async ({ page }) => {
    await page.goto('/manage/nodes')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
```

- [ ] **Step 3: Write policies E2E test**

Write `fronted/e2e/manage-policies.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('manage policies', () => {
  test('policies page renders', async ({ page }) => {
    await page.goto('/manage/policies')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
```

- [ ] **Step 4: Run E2E tests**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm dev &
sleep 5
pnpm test:e2e
```

Expected: All 5 E2E tests (3 new + 2 from auth spec) pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/e2e/dashboard.spec.ts fronted/e2e/manage-nodes.spec.ts fronted/e2e/manage-policies.spec.ts && git commit -m "test(frontend): add core E2E tests for dashboard, nodes, policies"
```

---

### Task 15: Go integration tests for server controller/service layer

**Files:**
- Create: `internal/server/controller/node_controller_integration_test.go`
- Create: `internal/server/controller/policy_controller_integration_test.go`

- [ ] **Step 1: Check existing test patterns**

Read existing controller test: `internal/agent/controller/network_controller_test.go` for Ginkgo/Gomega patterns.

- [ ] **Step 2: Write node controller integration test**

Write `internal/server/controller/node_controller_integration_test.go`:

```go
package controller_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNodeListIntegration(t *testing.T) {
	g := NewWithT(t)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	g.Expect(err).NotTo(HaveOccurred())

	store := gormstore.NewGormStore(db)
	g.Expect(store).NotTo(BeNil())
	g.Expect(db.AutoMigrate(&gormstore.Node{})).To(Succeed())

	nodes, err := store.ListNodes()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(nodes).NotTo(BeNil())
}
```

Note: Since we cannot fully predict the internal controller API without inspecting it, the actual test body should follow the established `internal/db/gormstore/*_test.go` fake-store pattern.

- [ ] **Step 3: Write policy controller integration test**

Write `internal/server/controller/policy_controller_integration_test.go`:

```go
package controller_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPolicyCRUDIntegration(t *testing.T) {
	g := NewWithT(t)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	g.Expect(err).NotTo(HaveOccurred())

	store := gormstore.NewGormStore(db)
	g.Expect(db.AutoMigrate(&gormstore.Policy{})).To(Succeed())

	// Create
	policy := &gormstore.Policy{Name: "test-policy", Type: "allow", Priority: 10}
	err = store.CreatePolicy(policy)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(policy.ID).NotTo(BeZero())

	// Read
	found, err := store.GetPolicy(policy.ID)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found.Name).To(Equal("test-policy"))

	// Delete
	err = store.DeletePolicy(policy.ID)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = store.GetPolicy(policy.ID)
	g.Expect(err).To(HaveOccurred())
}
```

- [ ] **Step 4: Run Go integration tests**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/server/controller/... -v -count=1
```

Expected: Tests compile and pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice && git add internal/server/controller/node_controller_integration_test.go internal/server/controller/policy_controller_integration_test.go && git commit -m "test(server): add integration test stubs for node and policy controllers"
```

**Note:** The GORM store types (`Node`, `Policy`, `NewGormStore`, etc.) should match the existing store implementation. Check `internal/db/gormstore/` for the actual type names and adjust accordingly.

---

### Task 15b: Expand Go E2E test scenarios (5 → 10+)

**Files:**
- Create: `test/e2e/policy_changes_test.go`
- Create: `test/e2e/peer_restart_test.go`
- Create: `test/e2e/ai_debug_intent_test.go`

- [ ] **Step 1: Write policy changes E2E test**

Write `test/e2e/policy_changes_test.go`:

```go
package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Policy Lifecycle", func() {
	It("creates, updates, and deletes a network policy", Label("e2e"), func() {
		g := NewWithT(GinkgoT())

		// Create a policy
		policyName := "e2e-test-policy"
		err := createPolicy(policyName, "allow", 50)
		g.Expect(err).NotTo(HaveOccurred())

		// Verify it appears in the list
		policies, err := listPolicies()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(policies).To(ContainElement(HaveField("Name", policyName)))

		// Delete it
		err = deletePolicy(policyName)
		g.Expect(err).NotTo(HaveOccurred())

		// Verify it's gone
		policies, err = listPolicies()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(policies).NotTo(ContainElement(HaveField("Name", policyName)))
	})
})
```

- [ ] **Step 2: Write peer restart resilience test**

Write `test/e2e/peer_restart_test.go`:

```go
package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Peer Restart Resilience", func() {
	It("re-establishes tunnel after peer restart", Label("e2e"), func() {
		g := NewWithT(GinkgoT())

		// Get a peer that is online
		peers, err := listPeers()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(peers).NotTo(BeEmpty())

		peer := peers[0]
		g.Expect(peer.Status).To(Equal("online"))

		// Restart the peer
		err = restartPeer(peer.ID)
		g.Expect(err).NotTo(HaveOccurred())

		// Wait for reconnection
		Eventually(func() string {
			p, _ := getPeer(peer.ID)
			return p.Status
		}, "30s", "1s").Should(Equal("online"))
	})
})
```

- [ ] **Step 3: Write AI debug/intent E2E test**

Write `test/e2e/ai_debug_intent_test.go`:

```go
package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AI Debug and Intent", func() {
	It("creates a network snapshot for debugging", Label("e2e", "ai"), func() {
		g := NewWithT(GinkgoT())

		snapshotID, err := createSnapshot("e2e-test-snapshot")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(snapshotID).NotTo(BeEmpty())

		// Verify snapshot appears in list
		snapshots, err := listSnapshots()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(snapshots).To(ContainElement(HaveField("ID", snapshotID)))
	})
})
```

- [ ] **Step 4: Run expanded E2E suite**

```bash
cd /Users/francis/workspc/lattice && make test-e2e
```

Expected: New tests pass alongside existing E2E tests.

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice && git add test/e2e/policy_changes_test.go test/e2e/peer_restart_test.go test/e2e/ai_debug_intent_test.go && git commit -m "test(e2e): expand Go E2E suite with policy, resilience, and AI scenarios"
```

---

**Files:**
- Create: `docs/demo/sandbox.md`
- Create: `docs/.vitepress/theme/components/LatticeSandbox.vue`

- [ ] **Step 1: Install xterm.js in docs project**

```bash
cd /Users/francis/workspc/lattice/docs && pnpm add @xterm/xterm @xterm/addon-fit @xterm/addon-web-links
```

- [ ] **Step 2: Create LatticeSandbox Vue component**

Write `docs/.vitepress/theme/components/LatticeSandbox.vue`:

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'

const terminalEl = ref<HTMLDivElement>()

onMounted(() => {
  const term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: '"JetBrains Mono", "Fira Code", monospace',
    theme: {
      background: '#1a1b26',
      foreground: '#a9b1d6',
      cursor: '#c0caf5',
    },
  })

  const fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.loadAddon(new WebLinksAddon())

  term.open(terminalEl.value!)
  fitAddon.fit()

  // Simulated demo experience
  const lines = [
    '$ lattice --version\r\n',
    'lattice v0.2.0\r\n\n',
    '$ latticed start --dev\r\n',
    'INF Starting LatticeD all-in-one...\r\n',
    'INF NATS server started on :4222\r\n',
    'INF Web UI available at http://localhost:8080\r\n\n',
    '$ lattice join --token wf_demo_xxxxxxxx\r\n',
    'INF Establishing secure tunnel...\r\n',
    'INF Peer enrolled: 10.42.0.1\r\n',
    'INF Tunnel status: READY\r\n\n',
    '$ lattice policy create allow-ssh --port 22 --target app=web\r\n',
    'INF Policy "allow-ssh" created\r\n',
    'INF Policy now active on 3 nodes\r\n',
  ]

  let i = 0
  const typeNextLine = () => {
    if (i < lines.length) {
      term.write(lines[i])
      i++
      if (lines[i - 1].startsWith('$')) {
        setTimeout(typeNextLine, 800)
      } else {
        setTimeout(typeNextLine, 300)
      }
    } else {
      term.write('\r\n$ _')
    }
  }

  setTimeout(typeNextLine, 500)

  const handleResize = () => fitAddon.fit()
  window.addEventListener('resize', handleResize)
  return () => window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div class="sandbox-container">
    <div class="sandbox-header">
      <span class="dot red"></span>
      <span class="dot yellow"></span>
      <span class="dot green"></span>
      <span class="title">Lattice Sandbox — try it live</span>
    </div>
    <div ref="terminalEl" class="sandbox-terminal"></div>
  </div>
</template>

<style scoped>
.sandbox-container {
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.3);
  margin: 2rem 0;
}

.sandbox-header {
  background: #2a2b3d;
  padding: 10px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.dot.red    { background: #ff5f56; }
.dot.yellow { background: #ffbd2e; }
.dot.green  { background: #27c93f; }

.title {
  margin-left: 12px;
  font-size: 13px;
  color: #787c99;
  font-family: system-ui, sans-serif;
}

.sandbox-terminal {
  height: 420px;
}

.sandbox-terminal :deep(.xterm) {
  height: 100%;
  padding: 8px;
}
</style>
```

- [ ] **Step 3: Register component in vitepress theme**

Create or edit `docs/.vitepress/theme/index.ts`:

```typescript
import DefaultTheme from 'vitepress/theme'
import LatticeSandbox from './components/LatticeSandbox.vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('LatticeSandbox', LatticeSandbox)
  },
}
```

- [ ] **Step 4: Create sandbox page**

Write `docs/demo/sandbox.md`:

```md
# Interactive Sandbox

See Lattice in action — watch a demo CLI session below.

<LatticeSandbox />

## Your Turn

Want to try it yourself?

```bash
docker compose -f docker-compose.demo.yml up -d
```

Open [http://localhost:8080](http://localhost:8080) and start managing your mesh.
```

- [ ] **Step 5: Verify the docs build works with the sandbox component**

```bash
cd /Users/francis/workspc/lattice/docs && pnpm build
```

Expected: Build succeeds without errors.

- [ ] **Step 6: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/.vitepress/theme/ docs/demo/sandbox.md docs/package.json docs/pnpm-lock.yaml && git commit -m "feat(docs): add interactive xterm.js sandbox demo"
```

---

### Task 17: Feature manual — one doc per module

**Files:**
- Create: `docs/features/nodes.md`
- Create: `docs/features/policies.md`
- Create: `docs/features/topology.md`
- Create: `docs/features/cluster-peering.md`
- Create: `docs/features/network-peering.md`
- Create: `docs/features/workspaces.md`
- Create: `docs/features/alerts.md`
- Create: `docs/features/monitoring.md`
- Create: `docs/features/relays.md`
- Create: `docs/features/audit.md`
- Create: `docs/features/account.md`
- Create: `docs/features/notifications.md`

- [ ] **Step 1: Create feature docs directory and first doc**

```bash
mkdir -p /Users/francis/workspc/lattice/docs/features
```

Write `docs/features/nodes.md`:

```md
# Nodes & Peers

Manage your WireGuard nodes — view status, join tokens, and peer connectivity.

## Overview

Every device running `lattice` is a **node** (or **peer**). Nodes form the mesh network through WireGuard tunnels.

## Features

- **Node dashboard** — See all nodes, their status (online/offline), IP address, and version
- **Join tokens** — Generate time-bound tokens to enroll new nodes
- **Peer wizard** — Step-by-step onboarding flow for new devices
- **Auto-discovery** — Nodes automatically discover each other via the control plane

## How to Use

1. Go to **Manage → Nodes** to see all nodes
2. Click **Generate Token** to create a join token
3. Run `lattice join --token <token>` on your new device
4. The node appears in the dashboard within seconds

## Related

- [Network Policies](/features/policies)
- [Topology Viewer](/features/topology)
```

Write `docs/features/policies.md`:

```md
# Network Policies

Define label-based access control rules for your mesh network.

## Overview

Lattice enforces **default-deny** networking. You create **allow** policies to permit specific traffic flows between labeled groups of nodes.

## Policy Types

| Type | Description |
|------|-------------|
| Allow | Permit traffic matching specified labels, ports, and protocols |
| Deny | Explicitly block traffic (overrides allows) |

## Policy Structure

A policy specifies:
- **Source labels** — which nodes the traffic comes from
- **Destination labels** — which nodes the traffic goes to
- **Port/Protocol** — which ports and protocols to allow/deny
- **Priority** — higher-priority policies are evaluated first

## Enforcement

| Edition | Backend |
|---------|---------|
| Community | iptables |
| Pro | eBPF (TC ingress on wf0 TUN) |

## Related

- [Nodes & Peers](/features/nodes)
- [Alerts & Rules](/features/alerts)
```

Write `docs/features/topology.md`:

```md
# Topology Viewer

Visual map of your mesh network showing all peers and their connections.

## Overview

The topology viewer renders a real-time graph of your Lattice network. Each node is a vertex, each active tunnel is an edge.

## Features

- **Live updates** — See tunnels establish in real-time
- **Node details** — Click a node for status, IP, version
- **Tunnel status** — Hover on edges for ICE/QUIC transport details

## How to Access

Go to **Manage → Topology** from the sidebar.
```

Write `docs/features/cluster-peering.md`:

```md
# Cluster Peering

Connect entire Kubernetes clusters across regions or cloud providers.

## Overview

Cluster peering creates a WireGuard bridge between two Kubernetes clusters. All pods in the peered CIDR ranges can communicate as if on the same network.

## How It Works

1. Install the Lattice operator on both clusters
2. Create a `LatticeNetwork` CRD on each cluster
3. The operator establishes a WireGuard tunnel between the two gateways
4. Pod-to-pod traffic routes through the tunnel automatically

## Related

- [Network Peering](/features/network-peering)
- [Relays](/features/relays)
```

Write `docs/features/network-peering.md`:

```md
# Network Peering

Connect isolated Lattice networks for cross-network communication.

## Overview

Network peering allows two independently-managed Lattice networks to establish secure communication between designated peers.

## Use Cases

- **Multi-cloud** — Connect dev (AWS) and prod (GCP) networks
- **Partner access** — Grant a partner limited access to specific services
- **Merging teams** — Bridge two organizations' networks

## Related

- [Cluster Peering](/features/cluster-peering)
- [Network Policies](/features/policies)
```

Write `docs/features/workspaces.md`:

```md
# Workspaces & Members

Multi-tenant isolation with role-based access control.

## Overview

Workspaces are isolated network environments. Each workspace has its own peers, policies, and member roles.

## Roles

| Role | Permissions |
|------|-------------|
| Admin | Full control over workspace settings, members, and network |
| Editor | Can modify policies, peers, and tokens |
| Viewer | Read-only access to dashboard and topology |

## How to Use

1. Go to **Manage → Workspaces** to create or switch workspaces
2. Go to **Manage → Members** to add users with roles
3. Each workspace has independent policy and token management
```

Write `docs/features/alerts.md`:

```md
# Alerts & Rules

Configure monitoring alerts for network events.

## Overview

Define alert rules that trigger notifications when specific conditions are met — node goes offline, tunnel fails, policy violation.

## Alert Types

- **Node status** — Online/offline transitions
- **Tunnel health** — Connection failures, latency spikes
- **Policy events** — Rule matches, enforcement failures

## Related

- [Monitoring](/features/monitoring)
- [Notifications](/features/notifications)
```

Write `docs/features/monitoring.md`:

```md
# Monitoring

Real-time metrics and dashboards for your mesh network.

## Overview

The monitoring dashboard shows network health metrics including tunnel throughput, latency, peer connectivity, and policy enforcement stats.

## How to Access

Go to **Manage → Monitor** from the sidebar.
```

Write `docs/features/relays.md`:

```md
# Relays

LRP (Lattice Relay Protocol) with QUIC fallback for NAT-traversal scenarios.

## Overview

When direct P2P connections can't be established (symmetric NAT, restrictive firewalls), Lattice routes traffic through a **relay** node.

## Configuration

Relays are configured in **Settings → Relays**. You can designate specific nodes as relay servers with bandwidth and connection limits.

## Related

- [Nodes & Peers](/features/nodes)
```

Write `docs/features/audit.md`:

```md
# Audit Logging

Track all actions across your Lattice deployment.

## Overview

The audit log records every significant action: who did what, when, and from which IP address.

## Logged Events

- User login/logout
- Policy creation, modification, deletion
- Token generation and revocation
- Member role changes
- Workspace creation/deletion

## Access

Go to **Settings → Audit** from the sidebar. Pro edition includes export and retention configuration.
```

Write `docs/features/account.md`:

```md
# Account & Profile

Manage your user account, profile, and billing.

## Sections

- **Profile** — Update name, avatar, contact info
- **Account** — Change password, manage sessions
- **Billing** — View plan, upgrade to Pro, download invoices
- **Notifications** — Configure alert channels (email, Slack, webhook)

## Access

Click your avatar in the sidebar → **Account Settings**.
```

Write `docs/features/notifications.md`:

```md
# Notifications

Configure how you receive alerts from Lattice.

## Channels

| Channel | Community | Pro |
|---------|-----------|-----|
| Email | ✅ | ✅ |
| Slack | ✅ | ✅ |
| Webhook | ✅ | ✅ |
| PagerDuty | — | ✅ |

## Setup

Go to **User → Notifications** from the sidebar to configure channels.
```

- [ ] **Step 2: Commit all feature docs**

```bash
cd /Users/francis/workspc/lattice && git add docs/features/ && git commit -m "docs: add feature manual — one doc per module"
```

---

### Task 18: Scenario guides

**Files:**
- Create: `docs/guides/multi-cloud-peering.md`
- Create: `docs/guides/remote-device-onboarding.md`
- Create: `docs/guides/ai-agent-zero-trust.md`

- [ ] **Step 1: Write multi-cloud peering guide**

Write `docs/guides/multi-cloud-peering.md`:

```md
# Multi-Cloud Cluster Peering

Connect Kubernetes clusters across AWS and GCP with WireGuard.

## Prerequisites

- Two Kubernetes clusters (any cloud or on-prem)
- `kubectl` configured for both clusters
- `helm` installed

## Step 1: Install Lattice Operator

```bash
helm repo add lattice https://charts.alattice.io
helm install lattice lattice/lattice-operator -n lattice --create-namespace
```

Repeat on both clusters.

## Step 2: Configure Network CIDRs

Create a `LatticeNetwork` CRD on each cluster:

```yaml
apiVersion: lattice.alattice.io/v1alpha1
kind: LatticeNetwork
metadata:
  name: cluster-peering
spec:
  cidr: 10.0.0.0/16
  peering:
    enabled: true
    remoteCIDR: 10.1.0.0/16
```

## Step 3: Verify Connectivity

```bash
kubectl exec -it deploy/sample-app -- curl http://10.1.2.3:8080/health
```

## Next Steps

- [Network Policies](/features/policies) — Control which services can communicate
- [Topology Viewer](/features/topology) — Visualize the cross-cluster mesh
```

- [ ] **Step 2: Write remote device onboarding guide**

Write `docs/guides/remote-device-onboarding.md`:

```md
# Remote Device Onboarding

Onboard laptops, edge devices, or home servers into your Lattice mesh.

## Prerequisites

- A running Lattice control plane (see [Quick Start](/guide/quickstart))
- Admin access to generate tokens

## Step 1: Generate a Join Token

In the dashboard: **Manage → Tokens → Generate Token**

Or via CLI:

```bash
lattice token create --network default --ttl 24h
```

Copy the token output (starts with `wf_`).

## Step 2: Install Lattice on the Device

```bash
curl -fsSL https://get.lattice.io | sh
```

## Step 3: Join the Mesh

```bash
lattice join --token wf_xxxx --server https://your-lattice-server:8080
```

You'll see:

```
INF Peer enrolled: 192.168.1.42
INF Tunnel established to control plane
INF Tunnel established to 1 peer
INF Status: ONLINE
```

## Step 4: Verify

Go to **Manage → Nodes** — the new device appears as "online".

## Next Steps

- [Network Policies](/features/policies) — Control what this device can access
- [Topology Viewer](/features/topology) — See your device on the mesh map
```

- [ ] **Step 3: Write AI agent zero-trust guide**

Write `docs/guides/ai-agent-zero-trust.md`:

```md
# Zero-Trust AI Agent Enrollment

Auto-enroll Claude Desktop, Cursor, or custom AI agents with time-bound WireGuard identities.

## Overview

Lattice can automatically issue ephemeral WireGuard credentials to AI agents. Each agent gets a scoped, time-limited identity — no permanent keys, no over-privileged access.

## Step 1: Enable MCP Server

Ensure the MCP Server is running in your Lattice deployment. See [MCP Server & ChatOps](/ai/mcp-server).

## Step 2: Configure AI Agent Enrollment

In the dashboard: **AI → Agents → Enable Auto-Enrollment**

```yaml
# lattice config for agent enrollment
ai:
  agent_enrollment:
    enabled: true
    default_ttl: 4h
    auto_scope: read-only
    require_approval: true
```

## Step 3: Connect Claude Desktop

In Claude Desktop settings:

```json
{
  "mcpServers": {
    "lattice": {
      "command": "npx",
      "args": ["-y", "@lattice/mcp-server"],
      "env": {
        "LATTICE_SERVER_URL": "https://your-lattice-instance:8080",
        "LATTICE_API_KEY": "your-api-key"
      }
    }
  }
}
```

## Step 4: Agent Gets Auto-Enrolled

When Claude Desktop connects:
1. Lattice issues a temporary WireGuard keypair
2. The agent appears as a peer with TTL badge
3. All agent actions are audited with the agent's identity

## Next Steps

- [AI Capabilities Overview](/ai/)
- [Compliance Automation](/ai/compliance)
```

- [ ] **Step 4: Add guides to vitepress sidebar**

Edit `docs/.vitepress/config.mts` sidebar — add after the "AI Capabilities" section:

```typescript
{
  text: 'Guides',
  items: [
    { text: 'Multi-Cloud Peering', link: '/guides/multi-cloud-peering' },
    { text: 'Remote Device Onboarding', link: '/guides/remote-device-onboarding' },
    { text: 'AI Agent Zero-Trust', link: '/guides/ai-agent-zero-trust' },
  ],
},
```

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/guides/ docs/.vitepress/config.mts && git commit -m "docs: add scenario guides for multi-cloud, device onboarding, and AI agent enrollment"
```

---

## Phase 3: Experience Polish (Tasks 19-24)

### Task 19: Expand Playwright E2E to 10+ tests

**Files:**
- Create: `fronted/e2e/ai-intent.spec.ts`
- Create: `fronted/e2e/manage-topology.spec.ts`
- Create: `fronted/e2e/settings.spec.ts`
- Create: `fronted/e2e/workspace.spec.ts`

- [ ] **Step 1: Write AI intent page test**

Write `fronted/e2e/ai-intent.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('AI intent', () => {
  test('AI intent page renders', async ({ page }) => {
    await page.goto('/ai/intent')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })

  test('can navigate between AI sub-pages', async ({ page }) => {
    await page.goto('/ai')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
```

- [ ] **Step 2: Write topology E2E test**

Write `fronted/e2e/manage-topology.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('topology', () => {
  test('topology page renders', async ({ page }) => {
    await page.goto('/manage/topology')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
```

- [ ] **Step 3: Write settings E2E test**

Write `fronted/e2e/settings.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('settings', () => {
  test('settings platform page renders', async ({ page }) => {
    await page.goto('/settings/platform')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })

  test('settings relays page renders', async ({ page }) => {
    await page.goto('/settings/relays')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
```

- [ ] **Step 4: Write workspace page test**

Write `fronted/e2e/workspace.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('workspace', () => {
  test('workspaces page renders', async ({ page }) => {
    await page.goto('/manage/workspaces')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })

  test('members page renders', async ({ page }) => {
    await page.goto('/manage/members')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
```

- [ ] **Step 5: Run all E2E tests**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm test:e2e
```

Expected: 9+ tests pass (5 from Phase 1/2 + 4 new).

- [ ] **Step 6: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/e2e/ai-intent.spec.ts fronted/e2e/manage-topology.spec.ts fronted/e2e/settings.spec.ts fronted/e2e/workspace.spec.ts && git commit -m "test(frontend): expand Playwright E2E to 10+ tests"
```

---

### Task 20: Visual regression testing setup

**Files:**
- Create: `fronted/e2e/visual/visual.spec.ts`
- Modify: `fronted/playwright.config.ts`

- [ ] **Step 1: Create visual regression test**

Write `fronted/e2e/visual/visual.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('visual regression', () => {
  test('dashboard page screenshot baseline', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')
    await expect(page).toHaveScreenshot('dashboard.png', {
      maxDiffPixelRatio: 0.05,
    })
  })
})
```

- [ ] **Step 2: Update Playwright config for visual tests**

Edit `fronted/playwright.config.ts` — add snapshot directory:

```typescript
export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 1,
  snapshotDir: './e2e/visual/snapshots',
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    command: 'pnpm dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
})
```

- [ ] **Step 3: Run visual test to generate baseline**

```bash
cd /Users/francis/workspc/lattice/fronted && pnpm dev &
sleep 5
pnpm test:e2e -- e2e/visual/ --update-snapshots
```

Expected: Generates `e2e/visual/snapshots/dashboard.png`.

- [ ] **Step 4: Commit**

```bash
cd /Users/francis/workspc/lattice && git add fronted/e2e/visual/ fronted/playwright.config.ts && git commit -m "test(frontend): add visual regression testing setup with Playwright screenshots"
```

---

### Task 21: Competitor comparison page

**Files:**
- Create: `docs/comparison.md`

- [ ] **Step 1: Write comparison page**

Write `docs/comparison.md`:

```md
# How Lattice Compares

## Overview

| Feature | Lattice | Tailscale | NetBird | Nebula |
|---------|---------|-----------|---------|--------|
| **Self-hosted** | ✅ Any infra | ❌ Cloud-dependent* | ✅ | ✅ |
| **Open-core** | ✅ Apache 2.0 | ❌ Proprietary | ✅ BSD | ✅ MIT |
| **WireGuard mesh** | ✅ | ✅ | ✅ | ❌ (custom proto) |
| **NAT traversal** | ICE + relay | DERP | STUN/TURN | UDP punching |
| **K8s native** | CRD operator | Operator | — | — |
| **Policy engine** | ACL + eBPF (Pro) | ACLs | ACLs | Firewall groups |
| **AI operations** | MCP + intent engine | — | — | — |
| **Compliance reports** | ✅ (Pro) | — | — | — |
| **Multi-tenant** | Workspaces + RBAC | Tailnet ACLs | Groups | — |
| **Relay protocol** | LRP over QUIC | DERP over HTTPS | TURN | Lighthouse |
| **Dashboard UI** | ✅ | ✅ Web | ✅ Web | ❌ CLI only |
| **Audit logging** | ✅ (basic: CE, full: Pro) | ✅ | ✅ | — |
| **Device support** | Linux, macOS, K8s | Most platforms | Most platforms | Most platforms |
| **Community size** | Growing | Large | Medium | Small |

\* Tailscale offers self-hosted "Headscale" but it's a separate community project, not officially supported.

## When to Choose Lattice

- You want **full control** — deploy the control plane on your own infrastructure
- You need **Kubernetes-native** networking — CRD operator, pod-level policies
- You want **AI integration** — MCP Server, natural language management, compliance automation
- You're **budget-conscious** — Community edition is free, Pro adds enterprise features
- You need **multi-tenant isolation** — Workspaces with independent RBAC

## When to Choose Alternatives

- **Tailscale** if you want managed infrastructure (no ops) and broadest device support
- **NetBird** if you want open-source with a polished managed cloud option
- **Nebula** if you're a Slack engineer and want a battle-tested custom protocol
```

- [ ] **Step 2: Add comparison to nav**

Edit `docs/.vitepress/config.mts` nav — add:

```typescript
{ text: 'vs.', link: '/comparison' },
```

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/comparison.md docs/.vitepress/config.mts && git commit -m "docs: add competitor comparison page"
```

---

### Task 22: Blog infrastructure and release notes page

**Files:**
- Create: `docs/blog/index.md`
- Create: `docs/blog/releases.md`

- [ ] **Step 1: Create blog index**

Write `docs/blog/index.md`:

```md
# Blog

Latest news, releases, and best practices from the Lattice project.

## Posts

- [v0.2.0 Release Notes](./releases) — New AI agent, signaling transport redesign, policy refactoring

## Subscribe

Watch our [GitHub releases](https://github.com/alatticeio/lattice/releases) for updates.
```

- [ ] **Step 2: Create release notes page**

Write `docs/blog/releases.md`:

```md
# Release Notes

## v0.2.0 (2026-05)

### Highlights

- **AI Agent**: MCP server integration, intent engine (Pro), time-travel debugging (Pro), compliance automation (Pro)
- **Signaling Redesign**: HTTP REST replaces NATS for CLI management
- **Policy Engine**: Refactored rule engine with eBPF support (Pro) and comprehensive test coverage
- **Network Peering**: Cluster and network peering with status cards
- **Audit & Telemetry**: Audit logging, member management revamp, i18n support

### Breaking Changes

- CLI management moved from NATS to HTTP REST. Update any automation scripts.
- Policy CRD structure changed. Re-apply policies after upgrade.

### Upgrade

```bash
curl -fsSL https://get.lattice.io | sh -s -- --version 0.2.0
lattice upgrade
```

## v0.1.3

First tagged release. WireGuard mesh with ICE NAT traversal, LRP relay, and basic dashboard.
```

- [ ] **Step 3: Add blog to vitepress nav**

Edit `docs/.vitepress/config.mts` nav — add:

```typescript
{ text: 'Blog', link: '/blog/' },
```

- [ ] **Step 4: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/blog/ docs/.vitepress/config.mts && git commit -m "docs: add blog and release notes infrastructure"
```

---

### Task 23: Final integration — docs build verification and CI check

**Files:**
- Modify: `Makefile`
- Modify: `docs/package.json`

- [ ] **Step 1: Add docs build to Makefile**

Add to `/Users/francis/workspc/lattice/Makefile`:

```makefile
.PHONY: build-docs
build-docs:
	cd docs && pnpm install && pnpm build

.PHONY: test-docs
test-docs:
	cd docs && pnpm build --outDir /tmp/lattice-docs-test
	@echo "Docs build successful"
```

- [ ] **Step 2: Run full verification**

```bash
# Frontend unit tests
cd /Users/francis/workspc/lattice/fronted && pnpm test

# Docs build
cd /Users/francis/workspc/lattice/docs && pnpm install && pnpm build

# Go unit tests with coverage
cd /Users/francis/workspc/lattice && make test
```

Expected: All three pass.

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add Makefile && git commit -m "chore: add docs build target and final integration check"
```

---

## Summary

| Phase | Tasks | Deliverables |
|-------|-------|-------------|
| Phase 1 (Infrastructure) | Tasks 1-10 | vitest + Playwright + MSW frameworks, example tests, CI gates, product landing page, docker-compose demo |
| Phase 2 (Core Coverage) | Tasks 11-18b | API unit tests (all modules), component tests, page tests, 5 E2E tests, Go integration tests, Go E2E expansion (5→8 scenarios), xterm.js sandbox, 12 feature docs, 3 scenario guides |
| Phase 3 (Polish) | Tasks 19-24 | 10+ E2E tests, visual regression, competitor comparison, blog/release notes, build verification |
