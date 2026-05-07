# Testing & Feature Showcase — Design

Date: 2026-05-07 | Status: Approved

## 1. Problem Statement

Lattice has 30+ frontend pages, 21 API modules, and a growing set of features (networking, policies, AI, compliance, monitoring). Currently:

- **Testing gap**: Go backend has 54 unit test files, but the Vue frontend has zero tests and no test framework. E2E coverage is thin (5 scenarios). No CI gating.
- **Showcase gap**: Documentation exists (vitepress) but is incomplete. Users have no way to quickly understand the full feature set, try the product hands-on, or find content relevant to their role.

## 2. Goals

1. Establish comprehensive test coverage across frontend, backend, and E2E
2. Build a product-quality documentation and demo site that serves four audience types:
   - K8s platform engineers
   - DevOps/developers
   - Technical decision makers
   - AI/security explorers

## 3. Testing Architecture

### 3.1 Frontend Testing (0 → complete)

```
fronted/
  src/
    __tests__/
      components/**/*.spec.ts     # Vue Test Utils component tests
      pages/**/*.spec.ts          # Page-level logic tests
      api/**/*.spec.ts            # API layer tests (MSW mocked)
      composables/**/*.spec.ts    # Composable unit tests
      stores/**/*.spec.ts         # Pinia store tests
  e2e/                            # Playwright E2E
    specs/
      auth.spec.ts
      dashboard.spec.ts
      manage-nodes.spec.ts
      manage-policies.spec.ts
      ai-intent.spec.ts
      ...
```

| Layer | Tool | What It Tests |
|-------|------|---------------|
| Unit | vitest + MSW | API layer, composables, stores |
| Component | vitest + Vue Test Utils | UI components in isolation |
| Page | vitest + mocked stores | Page logic, user flows |
| E2E | Playwright | Critical user journeys end-to-end |

**Coverage strategy**: Core paths (login, node management, policy configuration, AI interaction) get E2E. Everything else gets component + API tests.

### 3.2 Go Backend — Plug Gaps

| Gap | Action |
|-----|--------|
| Integration tests thin | Add fake-client integration tests for server controller/service layers |
| E2E scenarios limited | Expand from 5 to 10+ scenarios (policy changes, peer restart, network isolation recovery, AI debug/intent flows) |
| No performance regression | Add benchmark regression gates |
| No coverage gate | Enforce minimum coverage in CI |

### 3.3 CI Pipeline

```
CI:
  unit (Go)     ──┐
  unit (Vue)    ──┼──→ parallel ──→ results + coverage report
  e2e (Go)      ──┤
  e2e (Playwright) ┘
```

Branch protection: coverage gate, e2e gate on PRs to `master`/`dev`.

## 4. Showcase Architecture

### 4.1 Docs Site Structure

```
lattice.io (vitepress)
├── Home (product landing)
│   ├── Hero: one-line positioning + architecture diagram
│   ├── Feature map (grouped by module)
│   ├── vs. competitors comparison
│   └── CTA: Quick Start / Live Demo
│
├── Docs
│   ├── Quick Start (10 minutes)
│   ├── Installation
│   ├── Feature Manual (one chapter per module)
│   ├── Configuration Reference
│   ├── API Reference
│   └── FAQ
│
├── Guides (scenario-based)
│   ├── Multi-cloud cluster peering
│   ├── Remote device onboarding
│   ├── Zero-trust AI agent access
│   ├── Compliance audit automation
│   └── Monitoring stack setup
│
├── Demo (interactive)
│   ├── In-browser sandbox (xterm.js terminal)
│   └── Recorded walkthroughs
│
└── Blog
    ├── Release notes
    └── Best practices
```

### 4.2 Role-Based Navigation

| Role | Entry Path | Key Content |
|------|-----------|-------------|
| K8s Platform Engineer | Home → Docs → Feature Manual | CRDs, policy engine, topology, cluster peering |
| DevOps/Developer | Quick Start → Demo → Guides | One-command deploy, terminal sandbox, scenario guides |
| Decision Maker | Home → Feature Map → Comparison | Capability matrix, case studies, vs. Tailscale/Netbird |
| AI Explorer | Home → AI Section | MCP Server, AI ChatOps, compliance automation |

### 4.3 Interactive Demo

- `docker-compose up` script for all-in-one `latticed`
- In-browser terminal (xterm.js) guides user through `lattice` CLI
- Live feedback: topology updates, policy enforcement visible in real-time

Goal: user feels the product value in 5 minutes without installing anything locally.

## 5. Implementation Phases

### Phase 1: Infrastructure (1-2 weeks)

**Testing:**
- Install vitest + Vue Test Utils + Playwright in frontend
- Write 3-5 example tests to establish patterns
- Add coverage gate to Go CI

**Showcase:**
- Redesign vitepress landing page (product-style)
- Build feature map listing all modules
- Create `docker-compose` demo deployment

### Phase 2: Core Coverage (2-4 weeks)

**Testing:**
- Component tests for all critical pages
- Unit tests for API layer (`api/`)
- 3 Playwright E2E cases (login → create network → add node)
- Go integration test expansion

**Showcase:**
- One doc page per feature module
- 3-5 scenario guides
- Role-based navigation live

### Phase 3: Experience Polish (2-4 weeks)

**Testing:**
- Expand Playwright E2E to 10+ cases
- Visual regression testing (component screenshots)

**Showcase:**
- Embed live sandbox (xterm.js)
- Record walkthrough videos
- Blog + release notes page
- Competitor comparison page

**Total estimated duration**: 6-10 weeks.

## 6. Key Decisions

1. **vitest over jest**: vitest has native Vite support — no config duplication, faster execution, same transform pipeline as dev/prod.
2. **Playwright over Cypress**: faster execution, better parallelization, multi-browser support, better DX.
3. **MSW over axios interceptors**: Service Worker mocking is more realistic (network-level interception), and tests don't need to know about axios internals.
4. **xterm.js over cloud sandbox**: Self-contained, no external infrastructure, works with the existing all-in-one deployment model.
5. **One doc per page**: Each frontend feature page maps to a documentation entry, making docs complete and auditable.
