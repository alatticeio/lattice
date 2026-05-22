# Frontend Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign Lattice landing page and apply indigo/cyan theme globally via centralized CSS variables, with 4 reusable components.

**Architecture:** Single `style.css` controls all colors through CSS custom properties. Four new Vue components under `components/lattice/` provide the visual anchors (terminal, topology canvas, stat cards, section headers). The Home page is rewritten to use these components. Dashboard pages stay structurally identical but pick up the new theme automatically.

**Tech Stack:** Vue 3.5 + Vite + Tailwind 4 + shadcn-vue + TypeScript

---

## File Structure

```
fronted/src/
├── style.css                              # MODIFY: color vars + @layer lattice
├── components/lattice/
│   ├── LatticeTerminal.vue                # CREATE
│   ├── TopologyCanvas.vue                 # CREATE
│   ├── StatCard.vue                       # CREATE
│   └── SectionHeader.vue                  # CREATE
├── pages/
│   ├── index.vue                          # MODIFY: full rewrite
│   ├── dashboard/index.vue                # MODIFY: StatCard replacement
│   ├── sandbox/index.vue                  # MODIFY: lattice-card
│   ├── sandbox/AgentDetailDrawer.vue      # MODIFY: lattice-badge
│   └── manage/nodes/index.vue             # MODIFY: lattice-badge status
└── locales/
    ├── zh-CN/landing.json                 # MODIFY: new keys
    └── en/landing.json                    # MODIFY: new keys
```

---

### Task 1: CSS Variables — Color Palette Override

**Files:**
- Modify: `fronted/src/style.css`

- [ ] **Step 1: Replace all `:root` CSS variables with indigo/cyan palette**

Replace lines 46-79 of `style.css` (the entire `:root` block):

```css
:root {
  --radius: 0.625rem;
  --background: oklch(0.985 0.002 260);
  --foreground: oklch(0.145 0.005 260);
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.145 0.005 260);
  --popover: oklch(1 0 0);
  --popover-foreground: oklch(0.145 0.005 260);
  --primary: oklch(0.45 0.18 270);
  --primary-foreground: oklch(0.99 0 0);
  --secondary: oklch(0.95 0.01 265);
  --secondary-foreground: oklch(0.35 0.14 270);
  --muted: oklch(0.965 0.003 260);
  --muted-foreground: oklch(0.5 0.01 260);
  --accent: oklch(0.92 0.04 210);
  --accent-foreground: oklch(0.3 0.08 210);
  --destructive: oklch(0.58 0.24 27);
  --border: oklch(0.91 0.008 262);
  --input: oklch(0.91 0.008 262);
  --ring: oklch(0.5 0.15 270);
  --chart-1: oklch(0.55 0.18 270);
  --chart-2: oklch(0.6 0.14 210);
  --chart-3: oklch(0.5 0.12 230);
  --chart-4: oklch(0.65 0.2 290);
  --chart-5: oklch(0.7 0.15 180);
  --sidebar: oklch(0.97 0.003 260);
  --sidebar-foreground: oklch(0.145 0.005 260);
  --sidebar-primary: oklch(0.45 0.18 270);
  --sidebar-primary-foreground: oklch(0.99 0 0);
  --sidebar-accent: oklch(0.93 0.01 265);
  --sidebar-accent-foreground: oklch(0.35 0.14 270);
  --sidebar-border: oklch(0.91 0.008 262);
  --sidebar-ring: oklch(0.5 0.15 270);
}
```

- [ ] **Step 2: Replace `.dark` block with indigo-symmetric dark palette**

Replace lines 81-113 of `style.css` (the entire `.dark` block):

```css
.dark {
  --background: oklch(0.145 0.005 260);
  --foreground: oklch(0.985 0 0);
  --card: oklch(0.18 0.005 260);
  --card-foreground: oklch(0.985 0 0);
  --popover: oklch(0.18 0.005 260);
  --popover-foreground: oklch(0.985 0 0);
  --primary: oklch(0.7 0.16 270);
  --primary-foreground: oklch(0.145 0.005 260);
  --secondary: oklch(0.25 0.01 265);
  --secondary-foreground: oklch(0.985 0 0);
  --muted: oklch(0.25 0.01 265);
  --muted-foreground: oklch(0.65 0.02 260);
  --accent: oklch(0.25 0.04 210);
  --accent-foreground: oklch(0.985 0 0);
  --destructive: oklch(0.6 0.24 22);
  --border: oklch(1 0 0 / 10%);
  --input: oklch(1 0 0 / 15%);
  --ring: oklch(0.5 0.15 270);
  --chart-1: oklch(0.5 0.2 270);
  --chart-2: oklch(0.55 0.15 210);
  --chart-3: oklch(0.45 0.12 230);
  --chart-4: oklch(0.6 0.2 290);
  --chart-5: oklch(0.65 0.15 180);
  --sidebar: oklch(0.16 0.005 260);
  --sidebar-foreground: oklch(0.985 0 0);
  --sidebar-primary: oklch(0.7 0.16 270);
  --sidebar-primary-foreground: oklch(0.145 0.005 260);
  --sidebar-accent: oklch(0.22 0.01 265);
  --sidebar-accent-foreground: oklch(0.985 0 0);
  --sidebar-border: oklch(1 0 0 / 10%);
  --sidebar-ring: oklch(0.5 0.15 270);
}
```

- [ ] **Step 3: Add Inter font import alongside existing JetBrains Mono**

Replace the first line of `style.css`:

```css
@import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600;700&family=Inter:wght@400;500;600;700;800;900&display=swap');
```

- [ ] **Step 4: Add `--font-sans` and `--font-mono` to `@theme inline` block**

Add two lines inside the existing `@theme inline { }` block (after `--color-sidebar-ring`):

```css
  --font-sans: 'Inter', ui-sans-serif, system-ui;
  --font-mono: 'JetBrains Mono', ui-monospace;
```

- [ ] **Step 5: Verify by running dev server**

```bash
cd fronted && pnpm dev
```

Open `http://localhost:5173/dashboard` — sidebar and all shadcn components should use indigo instead of neutral gray. Check both light and dark modes.

- [ ] **Step 6: Commit**

```bash
git add fronted/src/style.css
git commit -m "feat(theme): replace neutral gray palette with indigo/cyan Lattice brand colors"
```

---

### Task 2: @layer lattice Utility Classes

**Files:**
- Modify: `fronted/src/style.css`

- [ ] **Step 1: Append `@layer lattice` block after the `.dark` block (before `@layer base`)**

Insert between `.dark { }` closing brace and `@layer base {`:

```css
@layer lattice {
  /* ── 终端组件 ───────────────────────────────── */
  .lattice-terminal {
    font-family: var(--font-mono);
    font-size: 0.875rem;
    line-height: 1.75;
    background: oklch(0.15 0.01 260);
    color: oklch(0.85 0.02 180);
    border-radius: var(--radius-xl);
    overflow: hidden;
  }
  .lattice-terminal .prompt {
    color: oklch(0.75 0.15 270);
  }
  .lattice-terminal .cmd {
    color: oklch(0.9 0.02 180);
  }
  .lattice-terminal .ok {
    color: oklch(0.7 0.16 160);
  }
  .lattice-terminal .warn {
    color: oklch(0.75 0.12 80);
  }
  .lattice-terminal .dim {
    color: oklch(0.5 0.01 260);
  }

  /* ── 渐变文字 ───────────────────────────────── */
  .lattice-gradient-text {
    font-weight: 900;
    letter-spacing: -0.04em;
    background: linear-gradient(135deg,
      oklch(0.45 0.18 270) 0%,
      oklch(0.55 0.16 240) 50%,
      oklch(0.6 0.14 210) 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  /* ── Lattice 卡片 ───────────────────────────── */
  .lattice-card {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    box-shadow: 0 1px 2px oklch(0.45 0.18 270 / 0.04);
    transition: box-shadow 0.2s, transform 0.2s;
  }
  .lattice-card:hover {
    box-shadow: 0 4px 12px oklch(0.45 0.18 270 / 0.08);
    transform: translateY(-1px);
  }

  /* ── 拓扑线 ─────────────────────────────────── */
  .lattice-topology-line {
    stroke: oklch(0.45 0.18 270 / 0.25);
    stroke-width: 1;
    fill: none;
  }
  .lattice-topology-node {
    fill: oklch(0.45 0.18 270 / 0.5);
  }

  /* ── 状态标签 ───────────────────────────────── */
  .lattice-badge-stable {
    font-size: 0.75rem;
    font-weight: 700;
    padding: 0.125rem 0.5rem;
    border-radius: 9999px;
    background: oklch(0.7 0.16 160 / 0.1);
    color: oklch(0.45 0.12 160);
  }
  .lattice-badge-roadmap {
    font-size: 0.75rem;
    font-weight: 700;
    padding: 0.125rem 0.5rem;
    border-radius: 9999px;
    background: oklch(0.75 0.12 80 / 0.1);
    color: oklch(0.5 0.1 80);
  }
  .lattice-badge-pro {
    font-size: 0.75rem;
    font-weight: 700;
    padding: 0.125rem 0.5rem;
    border-radius: 9999px;
    background: linear-gradient(135deg, oklch(0.45 0.18 270 / 0.1), oklch(0.6 0.14 210 / 0.1));
    color: oklch(0.45 0.18 270);
  }

  /* ── 数字/统计大字 ──────────────────────────── */
  .lattice-stat-number {
    font-family: var(--font-mono);
    font-weight: 900;
    font-size: 1.875rem;
    letter-spacing: -0.04em;
  }
}
```

Note: Tailwind 4's `@layer` does not support `@apply` inside it the same way Tailwind 3 does. All utility classes must use raw CSS properties, not `@apply` directives.

- [ ] **Step 2: Verify the utility classes are available**

```bash
cd fronted && pnpm dev
```

Add a temporary div to any page with class `lattice-gradient-text` and verify the gradient text rendering works. Remove the test div.

- [ ] **Step 3: Commit**

```bash
git add fronted/src/style.css
git commit -m "feat(theme): add @layer lattice utility classes (terminal, card, badge, gradient)"
```

---

### Task 3: LatticeTerminal Component

**Files:**
- Create: `fronted/src/components/lattice/LatticeTerminal.vue`

- [ ] **Step 1: Create the component directory and file**

```bash
mkdir -p fronted/src/components/lattice
```

- [ ] **Step 2: Write LatticeTerminal.vue**

```vue
<script setup lang="ts">
export interface TerminalLine {
  text: string
  cls?: 'prompt' | 'cmd' | 'ok' | 'warn' | 'dim'
}

defineProps<{
  title: string
  lines: TerminalLine[]
  status?: string
}>()
</script>

<template>
  <div class="lattice-terminal border border-white/10 shadow-2xl shadow-black/30">
    <!-- Title bar -->
    <div class="flex items-center gap-1.5 px-4 py-2.5 bg-black/30 border-b border-white/10">
      <div class="size-3 rounded-full bg-rose-500/70" />
      <div class="size-3 rounded-full bg-amber-400/70" />
      <div class="size-3 rounded-full bg-emerald-500/70" />
      <span class="ml-2 text-xs text-white/40 font-mono flex-1">{{ title }}</span>
      <template v-if="status">
        <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
        <span class="text-xs text-emerald-400 font-mono font-semibold">{{ status }}</span>
      </template>
    </div>
    <!-- Content -->
    <div class="p-5 font-mono text-sm leading-7 overflow-x-auto">
      <p v-for="(line, i) in lines" :key="i">
        <span :class="line.cls ?? 'cmd'">{{ line.text }}</span>
      </p>
    </div>
  </div>
</template>
```

- [ ] **Step 3: Verify component compiles**

```bash
cd fronted && pnpm build 2>&1 | tail -5
```

Expected: build success, no TypeScript errors.

- [ ] **Step 4: Commit**

```bash
git add fronted/src/components/lattice/LatticeTerminal.vue
git commit -m "feat(ui): add LatticeTerminal component with macOS-style chrome"
```

---

### Task 4: TopologyCanvas Component

**Files:**
- Create: `fronted/src/components/lattice/TopologyCanvas.vue`

- [ ] **Step 1: Write TopologyCanvas.vue**

```vue
<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

interface Node {
  id: number
  x: number
  y: number
  r: number
  vx: number
  vy: number
}

interface Link {
  source: number
  target: number
}

const props = withDefaults(defineProps<{
  nodeCount?: number
  animated?: boolean
}>(), {
  nodeCount: 12,
  animated: true,
})

const svgRef = ref<SVGSVGElement>()

interface SimNode extends Node {
  vx: number
  vy: number
}

const nodes = ref<SimNode[]>([])
const links = ref<Link[]>([])

let raf = 0

function initGraph(w: number, h: number) {
  const ns: SimNode[] = []
  const ls: Link[] = []

  for (let i = 0; i < props.nodeCount; i++) {
    ns.push({
      id: i,
      x: Math.random() * w,
      y: Math.random() * h,
      r: 3 + Math.random() * 4,
      vx: (Math.random() - 0.5) * 0.3,
      vy: (Math.random() - 0.5) * 0.3,
    })
  }

  for (let i = 0; i < ns.length; i++) {
    const j = (i + 1) % ns.length
    ls.push({ source: i, target: j })
    if (Math.random() > 0.5 && i + 2 < ns.length) {
      ls.push({ source: i, target: i + 2 })
    }
  }

  nodes.value = ns
  links.value = ls
}

function tick() {
  if (!props.animated) return
  const ns = nodes.value
  const margin = 40
  const w = svgRef.value?.clientWidth ?? 800
  const h = svgRef.value?.clientHeight ?? 600

  for (const n of ns) {
    n.x += n.vx
    n.y += n.vy
    if (n.x < margin || n.x > w - margin) n.vx *= -1
    if (n.y < margin || n.y > h - margin) n.vy *= -1
  }
  raf = requestAnimationFrame(tick)
}

onMounted(() => {
  const w = svgRef.value?.clientWidth ?? 800
  const h = svgRef.value?.clientHeight ?? 600
  initGraph(w, h)
  if (props.animated) raf = requestAnimationFrame(tick)
})

onUnmounted(() => cancelAnimationFrame(raf))
</script>

<template>
  <svg ref="svgRef" class="w-full h-full" viewBox="0 0 800 600" preserveAspectRatio="xMidYMid slice">
    <line
      v-for="(l, i) in links"
      :key="'l' + i"
      :x1="nodes[l.source]?.x ?? 0"
      :y1="nodes[l.source]?.y ?? 0"
      :x2="nodes[l.target]?.x ?? 0"
      :y2="nodes[l.target]?.y ?? 0"
      class="lattice-topology-line"
    />
    <circle
      v-for="n in nodes"
      :key="n.id"
      :cx="n.x"
      :cy="n.y"
      :r="n.r"
      class="lattice-topology-node"
    >
      <animate
        attributeName="r"
        :values="`${n.r};${n.r + 1.5};${n.r}`"
        :dur="2 + n.id * 0.3"
        repeatCount="indefinite"
      />
    </circle>
  </svg>
</template>
```

- [ ] **Step 2: Verify component compiles**

```bash
cd fronted && pnpm build 2>&1 | tail -5
```

- [ ] **Step 3: Commit**

```bash
git add fronted/src/components/lattice/TopologyCanvas.vue
git commit -m "feat(ui): add TopologyCanvas component with animated force graph"
```

---

### Task 5: StatCard Component

**Files:**
- Create: `fronted/src/components/lattice/StatCard.vue`

- [ ] **Step 1: Write StatCard.vue**

```vue
<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  icon: Component
  label: string
  value: number | string
  color: 'indigo' | 'emerald' | 'amber' | 'violet' | 'cyan'
}>()

const colorMap: Record<string, { bg: string; icon: string; num: string }> = {
  indigo:  { bg: 'bg-indigo-500/10',   icon: 'text-indigo-500',   num: 'text-indigo-600 dark:text-indigo-400' },
  emerald: { bg: 'bg-emerald-500/10',  icon: 'text-emerald-500',  num: 'text-emerald-600 dark:text-emerald-400' },
  amber:   { bg: 'bg-amber-500/10',    icon: 'text-amber-500',    num: 'text-amber-600 dark:text-amber-400' },
  violet:  { bg: 'bg-violet-500/10',   icon: 'text-violet-500',   num: 'text-violet-600 dark:text-violet-400' },
  cyan:    { bg: 'bg-cyan-500/10',     icon: 'text-cyan-500',     num: 'text-cyan-600 dark:text-cyan-400' },
}
</script>

<template>
  <div class="lattice-card p-6">
    <div class="flex items-center gap-4">
      <div class="size-10 rounded-xl flex items-center justify-center" :class="colorMap[color].bg">
        <component :is="icon" class="size-5" :class="colorMap[color].icon" />
      </div>
      <div>
        <p class="text-xs text-muted-foreground font-medium">{{ label }}</p>
        <p class="lattice-stat-number mt-0.5" :class="colorMap[color].num">{{ value }}</p>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Verify component compiles**

```bash
cd fronted && pnpm build 2>&1 | tail -5
```

- [ ] **Step 3: Commit**

```bash
git add fronted/src/components/lattice/StatCard.vue
git commit -m "feat(ui): add StatCard component for dashboard metrics"
```

---

### Task 6: SectionHeader Component

**Files:**
- Create: `fronted/src/components/lattice/SectionHeader.vue`

- [ ] **Step 1: Write SectionHeader.vue**

```vue
<script setup lang="ts">
defineProps<{
  tag: string
  title: string
  subtitle?: string
}>()
</script>

<template>
  <div class="text-center mb-12">
    <p class="text-xs font-black uppercase tracking-widest text-muted-foreground mb-2">{{ tag }}</p>
    <h2 class="text-2xl md:text-3xl font-black tracking-tighter text-foreground">{{ title }}</h2>
    <p v-if="subtitle" class="text-muted-foreground text-sm mt-2.5 max-w-lg mx-auto leading-relaxed">
      {{ subtitle }}
    </p>
  </div>
</template>
```

- [ ] **Step 2: Verify component compiles**

```bash
cd fronted && pnpm build 2>&1 | tail -5
```

- [ ] **Step 3: Commit**

```bash
git add fronted/src/components/lattice/SectionHeader.vue
git commit -m "feat(ui): add SectionHeader component for page section titles"
```

---

### Task 7: Home Page Rewrite

**Files:**
- Modify: `fronted/src/pages/index.vue`

- [ ] **Step 1: Rewrite index.vue — script section**

Replace the entire `<script setup>` block and `<template>` block. The script section:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  ArrowRight, Zap, Globe, Shield,
  CheckCircle, ChevronRight, Crown, X, LogOut, LayoutDashboard,
  Network, Container, Cpu, Terminal, BookOpen, Code2, Workflow,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { storeToRefs } from 'pinia'
import { useUserStore } from '@/stores/user'
import LatticeTerminal from '@/components/lattice/LatticeTerminal.vue'
import TopologyCanvas from '@/components/lattice/TopologyCanvas.vue'
import SectionHeader from '@/components/lattice/SectionHeader.vue'
import type { TerminalLine } from '@/components/lattice/LatticeTerminal.vue'

definePage({ meta: { layout: 'blank' } })

const { t } = useI18n()
const router = useRouter()
const userStore = useUserStore()
const { userInfo } = storeToRefs(userStore)
const { logout } = userStore

const avatarFallback = computed(() => {
  const name = userInfo.value?.username ?? userInfo.value?.email ?? '?'
  return name.slice(0, 2).toUpperCase()
})

const terminalLines: TerminalLine[] = [
  { text: '$ lattice sandbox start --name my-agent --token lt-enroll-xxx', cls: 'prompt' },
  { text: '  → NATS enrollment...                                              ✓', cls: 'ok' },
  { text: '  → WireGuard keypair generated                                     ✓', cls: 'ok' },
  { text: '  → VPN IP assigned: 10.100.0.5                                    ✓', cls: 'ok' },
  { text: '  Agent "my-agent" online, ICE P2P connected', cls: 'cmd' },
  { text: '', cls: 'dim' },
  { text: '# Tool call trace (tool_spans)', cls: 'dim' },
  { text: '$ ExecuteTool("list_peers")', cls: 'prompt' },
  { text: '  → CheckToolAccess ✓ → execute → 38ms', cls: 'ok' },
  { text: '  → tool_span: traceId=a1b2, status=ok', cls: 'cmd' },
  { text: '', cls: 'dim' },
  { text: '# RBAC enforcement (blocked)', cls: 'dim' },
  { text: '$ ExecuteTool("delete_peer")', cls: 'prompt' },
  { text: '  → CheckToolAccess ✗ → blocked', cls: 'warn' },
  { text: '  → tool_span: status=blocked → /audit/traces', cls: 'warn' },
]

const features = [
  { icon: Container, tag: 'stable', titleKey: 'landing.features.ai_sandbox.title', descKey: 'landing.features.ai_sandbox.desc' },
  { icon: Workflow, tag: 'stable', titleKey: 'landing.features.ai_traces.title', descKey: 'landing.features.ai_traces.desc' },
  { icon: Terminal, tag: 'stable', titleKey: 'landing.features.ai_mcp.title', descKey: 'landing.features.ai_mcp.desc' },
  { icon: Globe, tag: 'stable', titleKey: 'landing.features.net_wg.title', descKey: 'landing.features.net_wg.desc' },
  { icon: Cpu, tag: 'pro', titleKey: 'landing.features.net_ebpf.title', descKey: 'landing.features.net_ebpf.desc' },
  { icon: Zap, tag: 'pro', titleKey: 'landing.features.ai_intent.title', descKey: 'landing.features.ai_intent.desc' },
]

const pillarNetwork = [
  { icon: Globe, titleKey: 'landing.pillars.net_1_title', descKey: 'landing.pillars.net_1_desc' },
  { icon: Network, titleKey: 'landing.pillars.net_2_title', descKey: 'landing.pillars.net_2_desc' },
  { icon: Shield, titleKey: 'landing.pillars.net_3_title', descKey: 'landing.pillars.net_3_desc' },
  { icon: Code2, titleKey: 'landing.pillars.net_4_title', descKey: 'landing.pillars.net_4_desc' },
]

const pillarAgent = [
  { icon: Container, titleKey: 'landing.pillars.agent_1_title', descKey: 'landing.pillars.agent_1_desc' },
  { icon: Shield, titleKey: 'landing.pillars.agent_2_title', descKey: 'landing.pillars.agent_2_desc' },
  { icon: Workflow, titleKey: 'landing.pillars.agent_3_title', descKey: 'landing.pillars.agent_3_desc' },
  { icon: BookOpen, titleKey: 'landing.pillars.agent_4_title', descKey: 'landing.pillars.agent_4_desc' },
]

function tagClass(tag: string) {
  if (tag === 'stable') return 'lattice-badge-stable'
  if (tag === 'roadmap') return 'lattice-badge-roadmap'
  return 'lattice-badge-pro'
}

function tagLabel(tag: string) {
  if (tag === 'stable') return t('landing.features.tag_stable')
  if (tag === 'roadmap') return t('landing.features.tag_roadmap')
  return 'PRO'
}
</script>
```

- [ ] **Step 2: Write template — Navbar**

```vue
<template>
  <div class="min-h-screen bg-background text-foreground antialiased">

    <!-- ── Navbar ─────────────────────────────────────────────────── -->
    <header class="sticky top-0 z-50 border-b border-border bg-background/80 backdrop-blur-md">
      <div class="max-w-6xl mx-auto px-6 h-14 flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <img src="@/assets/logo.svg" class="size-7" alt="Lattice" />
          <span class="font-black tracking-tighter text-sm">Lattice</span>
          <span class="text-[10px] font-bold px-1.5 py-0.5 rounded-md bg-primary/10 text-primary ring-1 ring-primary/20">v0.1.2</span>
        </div>

        <nav class="hidden md:flex items-center gap-6 text-sm text-muted-foreground">
          <a href="#pillars"      class="hover:text-foreground transition-colors">{{ t('landing.nav.features') }}</a>
          <a href="#features"     class="hover:text-foreground transition-colors">Features</a>
          <a href="#quickstart"   class="hover:text-foreground transition-colors">{{ t('landing.nav.quickstart') }}</a>
          <a href="#pricing"      class="hover:text-foreground transition-colors">{{ t('landing.nav.pricing') }}</a>
        </nav>

        <div class="flex items-center gap-2">
          <a href="https://github.com/alatticeio/lattice" target="_blank" rel="noopener noreferrer" aria-label="GitHub"
            class="text-muted-foreground hover:text-foreground transition-colors p-1.5 rounded-md hover:bg-muted">
            <svg class="size-4" viewBox="0 0 98 96" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
              <path fill-rule="evenodd" clip-rule="evenodd" d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z"/>
            </svg>
          </a>

          <template v-if="!userInfo">
            <Button variant="ghost" size="sm" class="text-muted-foreground" @click="router.push('/auth/login')">{{ t('landing.nav.login') }}</Button>
            <Button size="sm" class="gap-1.5" @click="router.push('/dashboard')">
              {{ t('landing.nav.console') }} <ArrowRight class="size-3.5" />
            </Button>
          </template>

          <template v-else>
            <Button size="sm" class="gap-1.5" @click="router.push('/dashboard')">
              {{ t('landing.nav.console') }} <ArrowRight class="size-3.5" />
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <button class="hover:ring-border flex items-center gap-2 rounded-lg px-1.5 py-1 transition-colors hover:ring-2 hover:bg-muted">
                  <Avatar class="size-7">
                    <AvatarFallback class="bg-primary text-primary-foreground text-xs font-semibold">
                      {{ avatarFallback }}
                    </AvatarFallback>
                  </Avatar>
                  <div class="hidden text-left md:block">
                    <p class="text-sm font-medium leading-none">{{ userInfo.username }}</p>
                  </div>
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent class="w-48" align="end">
                <div class="px-2 py-1.5">
                  <p class="text-sm font-medium">{{ userInfo.username }}</p>
                  <p class="text-muted-foreground text-xs">{{ userInfo.email }}</p>
                </div>
                <DropdownMenuSeparator />
                <DropdownMenuItem @click="router.push('/dashboard')">
                  <LayoutDashboard class="mr-2 size-4" />
                  <span>{{ t('landing.nav.console') }}</span>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem class="text-destructive focus:text-destructive" @click="logout()">
                  <LogOut class="mr-2 size-4" />
                  <span>{{ t('landing.nav.logout') }}</span>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </template>
        </div>
      </div>
    </header>
```

- [ ] **Step 3: Write template — Hero section**

```vue
    <!-- ── Hero ───────────────────────────────────────────────────── -->
    <section class="relative overflow-hidden pt-28 pb-24 px-6">
      <div class="absolute inset-0 -z-10 opacity-40">
        <TopologyCanvas :node-count="14" />
      </div>
      <div class="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-64 bg-primary/10 rounded-full blur-3xl -z-10" />

      <div class="max-w-3xl mx-auto text-center relative">
        <span class="lattice-badge-stable inline-flex items-center gap-1.5 mb-6">
          <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
          {{ t('landing.hero.badge') }}
        </span>

        <h1 class="text-4xl md:text-5xl font-black tracking-tighter leading-[1.1] mb-5">
          <span class="lattice-gradient-text">{{ t('landing.hero.title_line1') }}</span>
          <br />
          <span class="text-foreground">{{ t('landing.hero.title_line2') }}</span>
        </h1>

        <p class="text-muted-foreground text-base leading-relaxed max-w-xl mx-auto mb-8">
          {{ t('landing.hero.subtitle') }}
        </p>

        <div class="flex flex-col sm:flex-row items-center justify-center gap-3">
          <Button size="lg" class="gap-2 px-7 shadow-lg shadow-primary/20">
            <Zap class="size-4" /> {{ t('landing.hero.cta_primary') }}
          </Button>
          <Button variant="outline" size="lg" class="gap-2 px-7" @click="router.push('/dashboard')">
            {{ t('landing.hero.cta_secondary') }} <ChevronRight class="size-4" />
          </Button>
        </div>
      </div>
    </section>
```

- [ ] **Step 4: Write template — Terminal Demo section**

```vue
    <!-- ── Terminal Demo ──────────────────────────────────────────── -->
    <section class="px-6 pb-20">
      <div class="max-w-3xl mx-auto">
        <LatticeTerminal
          :title="t('landing.terminal.title')"
          :status="t('landing.terminal.status')"
          :lines="terminalLines"
        />
        <p class="text-center text-xs text-muted-foreground mt-4 font-mono">
          {{ t('landing.terminal.caption') }}
        </p>
      </div>
    </section>
```

- [ ] **Step 5: Write template — Two Pillars section**

```vue
    <!-- ── Two Pillars ────────────────────────────────────────────── -->
    <section id="pillars" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-5xl mx-auto">
        <SectionHeader
          :tag="t('landing.pillars.tag')"
          :title="t('landing.pillars.title')"
          :subtitle="t('landing.pillars.subtitle')"
        />

        <div class="grid md:grid-cols-2 gap-6">
          <!-- Network Orchestration -->
          <div class="lattice-card p-8">
            <div class="size-12 rounded-xl bg-primary/10 text-primary flex items-center justify-center mb-5">
              <Network class="size-6" />
            </div>
            <h3 class="text-lg font-bold mb-4">{{ t('landing.pillars.net_title') }}</h3>
            <ul class="space-y-3">
              <li v-for="item in pillarNetwork" :key="item.titleKey" class="flex items-start gap-3">
                <component :is="item.icon" class="size-4 text-primary shrink-0 mt-0.5" />
                <div>
                  <p class="text-sm font-semibold">{{ t(item.titleKey) }}</p>
                  <p class="text-xs text-muted-foreground">{{ t(item.descKey) }}</p>
                </div>
              </li>
            </ul>
          </div>

          <!-- AI Agent Sandbox -->
          <div class="lattice-card p-8">
            <div class="size-12 rounded-xl bg-accent text-accent-foreground flex items-center justify-center mb-5">
              <Container class="size-6" />
            </div>
            <h3 class="text-lg font-bold mb-4">{{ t('landing.pillars.agent_title') }}</h3>
            <ul class="space-y-3">
              <li v-for="item in pillarAgent" :key="item.titleKey" class="flex items-start gap-3">
                <component :is="item.icon" class="size-4 text-accent-foreground shrink-0 mt-0.5" />
                <div>
                  <p class="text-sm font-semibold">{{ t(item.titleKey) }}</p>
                  <p class="text-xs text-muted-foreground">{{ t(item.descKey) }}</p>
                </div>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </section>
```

- [ ] **Step 6: Write template — Features Grid section**

```vue
    <!-- ── Features Grid ──────────────────────────────────────────── -->
    <section id="features" class="py-20 px-6">
      <div class="max-w-5xl mx-auto">
        <SectionHeader
          :tag="t('landing.features.label')"
          :title="t('landing.features.title')"
          :subtitle="t('landing.features.subtitle')"
        />

        <div class="grid md:grid-cols-3 gap-4">
          <div v-for="f in features" :key="f.titleKey" class="lattice-card p-6">
            <div class="size-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center mb-4">
              <component :is="f.icon" class="size-5" />
            </div>
            <span :class="tagClass(f.tag)">{{ tagLabel(f.tag) }}</span>
            <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t(f.titleKey) }}</h3>
            <p class="text-xs text-muted-foreground leading-relaxed">{{ t(f.descKey) }}</p>
          </div>
        </div>
      </div>
    </section>
```

- [ ] **Step 7: Write template — Quickstart section**

```vue
    <!-- ── Quickstart ─────────────────────────────────────────────── -->
    <section id="quickstart" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-3xl mx-auto">
        <SectionHeader
          :tag="t('landing.quickstart.tag')"
          :title="t('landing.quickstart.title')"
          :subtitle="t('landing.quickstart.subtitle')"
        />

        <div class="space-y-3">
          <div class="lattice-card p-5 font-mono text-sm">
            <span class="text-primary font-bold">$</span>
            <span class="text-foreground ml-2">docker run -d --name lattice-k3s --privileged -p 8080:8080 ghcr.io/alatticeio/lattice-k3s:latest</span>
            <p class="text-muted-foreground text-xs mt-2 ml-4">{{ t('landing.quickstart.docker_hint') }}</p>
          </div>
          <div class="lattice-card p-5 font-mono text-sm">
            <span class="text-primary font-bold">$</span>
            <span class="text-foreground ml-2">kubectl apply -k https://github.com/alatticeio/lattice/config/lattice/overlays/all-in-one</span>
            <p class="text-muted-foreground text-xs mt-2 ml-4">{{ t('landing.quickstart.k8s_hint') }}</p>
          </div>
          <div class="lattice-card p-5 font-mono text-sm">
            <span class="text-primary font-bold">$</span>
            <span class="text-foreground ml-2">lattice sandbox start --name my-agent --server-url https://lattice.company.com --token lt-enroll-xxx</span>
            <p class="text-muted-foreground text-xs mt-2 ml-4">{{ t('landing.quickstart.sandbox_hint') }}</p>
          </div>
        </div>
      </div>
    </section>
```

- [ ] **Step 8: Write template — Pricing section**

```vue
    <!-- ── Pricing ───────────────────────────────────────────────── -->
    <section id="pricing" class="py-20 px-6">
      <div class="max-w-4xl mx-auto">
        <SectionHeader
          :tag="t('landing.pricing.label')"
          :title="t('landing.pricing.title')"
          :subtitle="t('landing.pricing.subtitle')"
        />

        <div class="grid md:grid-cols-2 gap-5">
          <!-- Community -->
          <div class="lattice-card p-8 flex flex-col">
            <div class="mb-6">
              <p class="text-sm font-black uppercase tracking-widest text-muted-foreground mb-3">{{ t('landing.pricing.community_name') }}</p>
              <div class="flex items-end gap-1.5 mb-2">
                <span class="text-4xl font-black tracking-tighter text-foreground">{{ t('landing.pricing.community_price') }}</span>
              </div>
              <p class="text-xs text-muted-foreground">{{ t('landing.pricing.community_desc') }}</p>
            </div>
            <ul class="space-y-3 mb-8 flex-1">
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.community_feat_1') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.community_feat_2') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.community_feat_3') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.community_feat_4') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.community_feat_5') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.community_feat_6') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.community_feat_7') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-muted-foreground/50 line-through"><X class="size-4 text-muted-foreground/30 shrink-0" />{{ t('landing.pricing.pro_feat_locked_1') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-muted-foreground/50 line-through"><X class="size-4 text-muted-foreground/30 shrink-0" />{{ t('landing.pricing.pro_feat_locked_2') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-muted-foreground/50 line-through"><X class="size-4 text-muted-foreground/30 shrink-0" />{{ t('landing.pricing.pro_feat_locked_3') }}</li>
            </ul>
            <a href="https://github.com/alatticeio/lattice" target="_blank" rel="noopener noreferrer">
              <Button variant="outline" class="w-full" size="lg">
                {{ t('landing.pricing.community_cta') }}
              </Button>
            </a>
          </div>

          <!-- Pro -->
          <div class="relative lattice-card p-8 flex flex-col ring-2 ring-primary shadow-lg shadow-primary/10">
            <div class="absolute -top-3.5 left-1/2 -translate-x-1/2">
              <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-primary text-primary-foreground text-xs font-bold shadow-sm">
                <Crown class="size-3" /> {{ t('landing.pricing.pro_badge') }}
              </span>
            </div>
            <div class="mb-6">
              <p class="text-sm font-black uppercase tracking-widest text-muted-foreground mb-3">{{ t('landing.pricing.pro_name') }}</p>
              <div class="flex items-end gap-1.5 mb-2">
                <span class="text-4xl font-black tracking-tighter text-foreground">{{ t('landing.pricing.pro_price') }}</span>
                <span class="text-sm text-muted-foreground mb-1.5">{{ t('landing.pricing.pro_period') }}</span>
              </div>
              <p class="text-xs text-muted-foreground">{{ t('landing.pricing.pro_desc') }}</p>
            </div>
            <ul class="space-y-3 mb-8 flex-1">
              <li class="flex items-center gap-2.5 text-sm font-medium text-primary"><CheckCircle class="size-4 shrink-0" />{{ t('landing.pricing.pro_feat_all') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.pro_feat_1') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.pro_feat_2') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.pro_feat_3') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.pro_feat_4') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.pro_feat_5') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.pro_feat_6') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.pro_feat_7') }}</li>
              <li class="flex items-center gap-2.5 text-sm text-foreground"><CheckCircle class="size-4 text-emerald-500 shrink-0" />{{ t('landing.pricing.pro_feat_8') }}</li>
            </ul>
            <Button class="w-full gap-2 shadow-md shadow-primary/20" size="lg" @click="router.push('/auth/login')">
              <Crown class="size-4" /> {{ t('landing.pricing.pro_cta') }}
            </Button>
            <p class="text-center text-xs text-muted-foreground mt-3">{{ t('landing.pricing.pro_disclaimer') }}</p>
          </div>
        </div>

        <div class="mt-5 flex items-center justify-center gap-2 text-sm text-muted-foreground">
          <span>{{ t('landing.pricing.enterprise_text') }}</span>
          <a href="mailto:hello@alattice.io" class="text-foreground font-medium hover:underline underline-offset-4 transition-colors">{{ t('landing.pricing.enterprise_link') }}</a>
        </div>
      </div>
    </section>
```

- [ ] **Step 9: Write template — CTA + Footer**

```vue
    <!-- ── CTA ────────────────────────────────────────────────────── -->
    <section class="py-20 px-6">
      <div class="max-w-xl mx-auto text-center">
        <div class="size-14 rounded-2xl bg-primary/10 flex items-center justify-center mx-auto mb-5">
          <Container class="size-7 text-primary" />
        </div>
        <h2 class="text-2xl font-black tracking-tighter mb-3 text-foreground">{{ t('landing.cta.title') }}</h2>
        <p class="text-muted-foreground text-sm leading-relaxed mb-7 max-w-sm mx-auto">
          {{ t('landing.cta.subtitle') }}
        </p>
        <div class="flex flex-col sm:flex-row gap-3 justify-center mb-8">
          <Button size="lg" class="gap-2 px-8 shadow-lg shadow-primary/20">
            <Zap class="size-4" /> {{ t('landing.cta.button_primary') }}
          </Button>
          <Button variant="outline" size="lg" class="gap-2 px-8" @click="router.push('/dashboard')">
            {{ t('landing.cta.button_secondary') }} <ArrowRight class="size-4" />
          </Button>
        </div>
        <div class="grid grid-cols-3 gap-2 text-left max-w-xs mx-auto">
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_1') }}</div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_2') }}</div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_3') }}</div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_4') }}</div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_5') }}</div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_6') }}</div>
        </div>
      </div>
    </section>

    <!-- ── Footer ─────────────────────────────────────────────────── -->
    <footer class="border-t border-border px-6 py-7">
      <div class="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
        <div class="flex items-center gap-2">
          <img src="@/assets/logo.svg" class="size-5" alt="Lattice" />
          <span class="text-sm font-black tracking-tighter text-muted-foreground">Lattice</span>
        </div>
        <p class="text-xs text-muted-foreground font-mono uppercase tracking-widest">
          {{ t('landing.footer.copyright') }}
        </p>
        <div class="flex items-center gap-5 text-xs text-muted-foreground">
          <a href="#" class="hover:text-foreground transition-colors">{{ t('landing.nav.docs') }}</a>
          <a href="#pricing" class="hover:text-foreground transition-colors">{{ t('landing.nav.pricing') }}</a>
          <a href="https://github.com/alatticeio/lattice" target="_blank" rel="noopener noreferrer" class="hover:text-foreground transition-colors">{{ t('landing.nav.github') }}</a>
          <a href="#" class="hover:text-foreground transition-colors">{{ t('landing.nav.community') }}</a>
          <a href="/legal/privacy" class="hover:text-foreground transition-colors">隐私政策</a>
          <a href="/legal/terms" class="hover:text-foreground transition-colors">服务条款</a>
        </div>
      </div>
    </footer>

  </div>
</template>
```

- [ ] **Step 10: Verify build**

```bash
cd fronted && pnpm build 2>&1 | tail -10
```

Expected: build success. Some i18n keys will be missing until Task 8 — that's expected.

- [ ] **Step 11: Commit**

```bash
git add fronted/src/pages/index.vue
git commit -m "feat(home): rewrite landing page with new components and indigo theme"
```

---

### Task 8: i18n Sync

**Files:**
- Modify: `fronted/src/locales/zh-CN/landing.json`
- Modify: `fronted/src/locales/en/landing.json`

- [ ] **Step 1: Write zh-CN/landing.json with all keys needed by new Home page**

```json
{
  "nav": {
    "features": "功能",
    "architecture": "架构",
    "pricing": "定价",
    "quickstart": "快速开始",
    "login": "登录",
    "console": "控制台",
    "logout": "退出",
    "docs": "文档",
    "github": "GitHub",
    "community": "社区"
  },
  "hero": {
    "badge": "AI-Native 网络",
    "title_line1": "AI Agent 的",
    "title_line2": "零信任运行环境",
    "subtitle": "一条命令启动沙箱，零特权隔离，端到端 WireGuard 加密，工具调用全程可追溯。为每个 AI Agent 赋予独立网络身份。",
    "cta_primary": "lattice sandbox start",
    "cta_secondary": "查看控制台"
  },
  "terminal": {
    "title": "lattice — sandbox",
    "status": "ONLINE",
    "caption": "NATS 注册 → WireGuard 密钥生成 → VPN IP 分配 → ICE P2P 连接 → 工具调用追踪"
  },
  "pillars": {
    "tag": "两大引擎",
    "title": "网络编排 + AI Agent 安全，一个二进制搞定",
    "subtitle": "Lattice 在一个统一底座上实现两件事：跨云网络互联，和 AI Agent 运行时防护。",
    "net_title": "网络编排引擎",
    "agent_title": "AI Agent 沙箱引擎",
    "net_1_title": "WireGuard 加密 Mesh",
    "net_1_desc": "密钥分发、IP 分配、隧道建立全程自动化",
    "net_2_title": "ICE/STUN NAT 穿透",
    "net_2_desc": "P2P 直连优先，LRP relay 自动回退",
    "net_3_title": "策略引擎",
    "net_3_desc": "声明式 allow/deny，iptables 或 eBPF TC 执行",
    "net_4_title": "K8s CRD 编排",
    "net_4_desc": "13 个 CRD，声明式管理网络生命周期",
    "agent_1_title": "gVisor 零特权沙箱",
    "agent_1_desc": "用户态网络栈，无需 root/TUN/iptables",
    "agent_2_title": "Zero-Trust 注册",
    "agent_2_desc": "一次性 enrollment token → Agent JWT，凭证持久化",
    "agent_3_title": "工具调用追踪 + 委派",
    "agent_3_desc": "tool_spans 可观测，Sub-agent 权限不超父级",
    "agent_4_title": "MCP Server",
    "agent_4_desc": "14 个工具，Claude/Cursor 自然语言管理网络"
  },
  "features": {
    "label": "全部能力",
    "title": "6 大核心能力",
    "subtitle": "Community 版免费使用全部基础能力，PRO 版解锁企业级安全特性。",
    "tag_stable": "稳定版",
    "tag_roadmap": "路线图",
    "ai_sandbox": {
      "title": "零特权沙箱",
      "desc": "gVisor pkg/tcpip 用户态网络栈，无 root 运行，完整 ICE/LRP 网络身份，凭证跨重启持久化。"
    },
    "ai_traces": {
      "title": "工具调用追踪",
      "desc": "每次 MCP 工具调用自动记录 tool_span（traceId、agentId、耗时、状态），Sub-agent 委派权限不超父级。"
    },
    "ai_mcp": {
      "title": "MCP Server",
      "desc": "内置 14 个工具，Claude/Cursor 等 AI 助手直接管理网络。读操作直接执行，写操作含审批工作流。"
    },
    "net_wg": {
      "title": "WireGuard Mesh",
      "desc": "NAT 穿透 P2P + LRP relay，端到端加密，ICE/STUN 自动打洞，跨集群桥接零配置，内置 IPAM。"
    },
    "net_ebpf": {
      "title": "eBPF 策略引擎",
      "desc": "内核级 TC ingress/egress，LPM Trie + Port Hash 百万规则匹配，基础设施节点线速转发（PRO）。"
    },
    "ai_intent": {
      "title": "意图引擎",
      "desc": "自然语言 → CRD 变更计划 → Diff 预览 → 审批执行，完整 human-in-the-loop 工作流（PRO）。"
    }
  },
  "quickstart": {
    "tag": "快速开始",
    "title": "三条命令，三种部署方式",
    "subtitle": "从 Docker 到 K8s 到 Sandbox，任选一种。",
    "docker_hint": "约 30 秒后访问 http://localhost:8080",
    "k8s_hint": "已有 K8s 集群时使用，自动创建所有 CRD 和控制器",
    "sandbox_hint": "零特权启动 AI Agent 沙箱，首次需 enrollment token"
  },
  "pricing": {
    "label": "定价",
    "title": "选择适合你的版本",
    "subtitle": "社区版永远免费，Pro 版解锁企业级安全能力。",
    "community_name": "社区版",
    "community_price": "免费",
    "community_desc": "适合个人开发者和 AI Agent 探索者",
    "community_cta": "在 GitHub 上查看",
    "community_feat_1": "WireGuard 加密 Mesh：无限节点",
    "community_feat_2": "CRD 原生 K8s 控制器（13 CRDs）",
    "community_feat_3": "NATS 信令 + ICE/STUN P2P + LRP Relay",
    "community_feat_4": "gVisor 零特权沙箱（无需 root / TUN / iptables）",
    "community_feat_5": "工具调用追踪（tool_spans）+ 审计查询 API",
    "community_feat_6": "Sub-agent 委派 API + 调用树",
    "community_feat_7": "MCP Server（14 工具，写操作含审批工作流）",
    "pro_name": "Pro",
    "pro_price": "联系我们",
    "pro_period": "",
    "pro_desc": "适合需要生产级安全的企业和团队",
    "pro_cta": "升级到 Pro",
    "pro_disclaimer": "30 天免费试用，无需信用卡",
    "pro_badge": "推荐",
    "pro_feat_all": "所有社区版功能",
    "pro_feat_1": "出站策略过滤（EgressFilter + CIDR 白名单）+ 入站端口转发 + HTTP 正向代理",
    "pro_feat_2": "NATS 中心化流量审计（flow_events）+ tool_spans 关联",
    "pro_feat_3": "eBPF TC 策略引擎（内核级 LPM/Port 策略）",
    "pro_feat_4": "意图引擎：自然语言 → CRD 变更计划 + Diff 预览 + 审批",
    "pro_feat_5": "K8s 集群互联（ClusterPeering）",
    "pro_feat_6": "SSO/OIDC + 高级 RBAC + 审批工作流",
    "pro_feat_7": "指标推送（VictoriaMetrics）+ Webhook 告警",
    "pro_feat_8": "Firecracker MicroVM 沙箱（路线图）",
    "pro_feat_locked_1": "出站策略过滤（EgressFilter）",
    "pro_feat_locked_2": "eBPF TC 策略引擎",
    "pro_feat_locked_3": "意图引擎（自然语言网络管理）",
    "enterprise_text": "需要大规模部署？",
    "enterprise_link": "联系企业销售 →"
  },
  "cta": {
    "title": "为你的 AI Agent 构建安全运行环境",
    "subtitle": "一条命令启动沙箱，零特权隔离，端到端加密，工具调用全程可追溯。",
    "button_primary": "lattice sandbox start",
    "button_secondary": "查看控制台",
    "badge_1": "零特权",
    "badge_2": "gVisor 隔离",
    "badge_3": "WireGuard 加密",
    "badge_4": "工具调用追踪",
    "badge_5": "Sub-agent 委派",
    "badge_6": "流量审计"
  },
  "footer": {
    "copyright": "© 2026 Lattice. All rights reserved."
  },
  "stats": {
    "active_nodes": "活跃沙箱",
    "all_healthy": "全部健康",
    "avg_latency": "平均延迟",
    "sync": "最后同步",
    "data_plane": "沙箱数据面"
  },
  "advantages": {
    "item_1": "零特权 — 普通用户运行",
    "item_2": "gVisor 用户态网络隔离",
    "item_3": "WireGuard 端到端加密",
    "item_4": "工具调用全程可追溯",
    "item_5": "Sub-agent 委派 + 调用树",
    "item_6": "自然语言网络管理（PRO）"
  }
}
```

- [ ] **Step 2: Write en/landing.json with all keys (same structure, English values)**

```json
{
  "nav": {
    "features": "Features",
    "architecture": "Architecture",
    "pricing": "Pricing",
    "quickstart": "Quickstart",
    "login": "Login",
    "console": "Console",
    "logout": "Logout",
    "docs": "Docs",
    "github": "GitHub",
    "community": "Community"
  },
  "hero": {
    "badge": "AI-Native Networking",
    "title_line1": "Zero-Trust Runtime",
    "title_line2": "for AI Agents",
    "subtitle": "One command to launch a sandbox. Zero privilege. WireGuard end-to-end encryption. Every tool call traced. Give every AI agent its own network identity.",
    "cta_primary": "lattice sandbox start",
    "cta_secondary": "View Console"
  },
  "terminal": {
    "title": "lattice — sandbox",
    "status": "ONLINE",
    "caption": "NATS enrollment → WireGuard keygen → VPN IP → ICE P2P → Tool tracing"
  },
  "pillars": {
    "tag": "Two Pillars",
    "title": "Network Orchestration + AI Agent Security, One Binary",
    "subtitle": "Lattice delivers two things on a unified foundation: cross-cloud connectivity and AI agent runtime protection.",
    "net_title": "Network Orchestration Engine",
    "agent_title": "AI Agent Sandbox Engine",
    "net_1_title": "WireGuard Encrypted Mesh",
    "net_1_desc": "Automated key distribution, IP allocation, tunnel setup",
    "net_2_title": "ICE/STUN NAT Traversal",
    "net_2_desc": "P2P direct connect, LRP relay automatic fallback",
    "net_3_title": "Policy Engine",
    "net_3_desc": "Declarative allow/deny, iptables or eBPF TC enforcement",
    "net_4_title": "K8s CRD Orchestration",
    "net_4_desc": "13 CRDs, declarative network lifecycle management",
    "agent_1_title": "gVisor Zero-Privilege Sandbox",
    "agent_1_desc": "Userspace netstack, no root/TUN/iptables required",
    "agent_2_title": "Zero-Trust Enrollment",
    "agent_2_desc": "One-time enrollment token → Agent JWT, credential persistence",
    "agent_3_title": "Tool Trace + Delegation",
    "agent_3_desc": "tool_spans observability, sub-agent scoped permissions",
    "agent_4_title": "MCP Server",
    "agent_4_desc": "14 tools, manage your network via Claude/Cursor in natural language"
  },
  "features": {
    "label": "Capabilities",
    "title": "6 Core Capabilities",
    "subtitle": "Community is free forever with all core features. PRO unlocks enterprise security.",
    "tag_stable": "Stable",
    "tag_roadmap": "Roadmap",
    "ai_sandbox": {
      "title": "Zero-Privilege Sandbox",
      "desc": "gVisor pkg/tcpip userspace netstack, no root, full ICE/LRP identity, credentials persist across restarts."
    },
    "ai_traces": {
      "title": "Tool Call Tracing",
      "desc": "Every MCP tool call records a tool_span (traceId, agentId, duration, status). Sub-agent delegation with scoped permissions."
    },
    "ai_mcp": {
      "title": "MCP Server",
      "desc": "14 built-in tools. Claude, Cursor, and other AI assistants manage your network. Reads execute, writes require approval."
    },
    "net_wg": {
      "title": "WireGuard Mesh",
      "desc": "NAT-traversing P2P + LRP relay, end-to-end encryption, ICE/STUN auto hole-punching, built-in IPAM."
    },
    "net_ebpf": {
      "title": "eBPF Policy Engine",
      "desc": "Kernel-level TC ingress/egress, LPM Trie + Port Hash, millions of rules, line-rate for infrastructure nodes (PRO)."
    },
    "ai_intent": {
      "title": "Intent Engine",
      "desc": "Natural language → CRD change plan → diff preview → approve → apply. Full human-in-the-loop workflow (PRO)."
    }
  },
  "quickstart": {
    "tag": "Quickstart",
    "title": "Three Commands, Three Ways to Deploy",
    "subtitle": "Docker, Kubernetes, or sandbox — pick yours.",
    "docker_hint": "Visit http://localhost:8080 after ~30s",
    "k8s_hint": "For existing clusters, auto-creates all CRDs and controllers",
    "sandbox_hint": "Zero-privilege AI agent sandbox, requires enrollment token on first launch"
  },
  "pricing": {
    "label": "Pricing",
    "title": "Choose Your Edition",
    "subtitle": "Community edition is free forever. Pro unlocks enterprise security.",
    "community_name": "Community",
    "community_price": "Free",
    "community_desc": "For individual developers and AI agent explorers",
    "community_cta": "View on GitHub",
    "community_feat_1": "WireGuard Encrypted Mesh: unlimited nodes",
    "community_feat_2": "CRD-Native K8s Operator (13 CRDs)",
    "community_feat_3": "NATS Signaling + ICE/STUN P2P + LRP Relay",
    "community_feat_4": "gVisor Zero-Privilege Sandbox (no root / TUN / iptables)",
    "community_feat_5": "Tool Call Tracing (tool_spans) + Audit Query API",
    "community_feat_6": "Sub-agent Delegation API + Call Tree",
    "community_feat_7": "MCP Server (14 tools, write ops require approval)",
    "pro_name": "Pro",
    "pro_price": "Contact Us",
    "pro_period": "",
    "pro_desc": "For teams and enterprises needing production-grade security",
    "pro_cta": "Upgrade to Pro",
    "pro_disclaimer": "30-day free trial, no credit card required",
    "pro_badge": "Recommended",
    "pro_feat_all": "Everything in Community",
    "pro_feat_1": "Egress policy filtering (EgressFilter + CIDR allowlist) + inbound port forwarding + HTTP proxy",
    "pro_feat_2": "Centralized NATS flow audit (flow_events) + tool_spans correlation",
    "pro_feat_3": "eBPF TC policy engine (kernel-level LPM/Port matching)",
    "pro_feat_4": "Intent Engine: natural language → CRD change plan + diff preview + approval",
    "pro_feat_5": "Kubernetes Cluster Peering (ClusterPeering CRD)",
    "pro_feat_6": "SSO/OIDC + advanced RBAC + approval workflows",
    "pro_feat_7": "Metrics push (VictoriaMetrics) + Webhook alerts",
    "pro_feat_8": "Firecracker MicroVM Sandbox (roadmap)",
    "pro_feat_locked_1": "Egress policy filtering (EgressFilter)",
    "pro_feat_locked_2": "eBPF TC policy engine",
    "pro_feat_locked_3": "Intent Engine (natural language network management)",
    "enterprise_text": "Need large-scale deployment?",
    "enterprise_link": "Contact Enterprise Sales →"
  },
  "cta": {
    "title": "Build a Secure Runtime for Your AI Agents",
    "subtitle": "One command to launch a sandbox. Zero privilege. End-to-end encryption. Every tool call traced.",
    "button_primary": "lattice sandbox start",
    "button_secondary": "View Console",
    "badge_1": "Zero Privilege",
    "badge_2": "gVisor Isolation",
    "badge_3": "WireGuard Encryption",
    "badge_4": "Tool Call Tracing",
    "badge_5": "Sub-agent Delegation",
    "badge_6": "Traffic Audit"
  },
  "footer": {
    "copyright": "© 2026 Lattice. All rights reserved."
  },
  "stats": {
    "active_nodes": "Active Sandboxes",
    "all_healthy": "All Healthy",
    "avg_latency": "Avg Latency",
    "sync": "Last Sync",
    "data_plane": "Sandbox Data Plane"
  },
  "advantages": {
    "item_1": "Zero-Privilege — runs as normal user",
    "item_2": "gVisor userspace network isolation",
    "item_3": "WireGuard end-to-end encryption",
    "item_4": "Full tool call traceability",
    "item_5": "Sub-agent delegation + call tree",
    "item_6": "Natural language network management (PRO)"
  }
}
```

- [ ] **Step 3: Verify build with all i18n keys present**

```bash
cd fronted && pnpm build 2>&1 | tail -10
```

Expected: build success, no i18n key warnings.

- [ ] **Step 4: Commit**

```bash
git add fronted/src/locales/zh-CN/landing.json fronted/src/locales/en/landing.json
git commit -m "feat(i18n): update landing page translations for new Home design"
```

---

### Task 9: Dashboard — StatCard Replacement + Badge Migration

**Files:**
- Modify: `fronted/src/pages/dashboard/index.vue`
- Modify: `fronted/src/pages/sandbox/index.vue`
- Modify: `fronted/src/pages/sandbox/AgentDetailDrawer.vue`
- Modify: `fronted/src/pages/manage/nodes/index.vue`

- [ ] **Step 1: Replace Dashboard stat cards with StatCard component**

In `fronted/src/pages/dashboard/index.vue`, replace the existing stat card grid. Import StatCard and use it instead of hand-coded stat cards. This is a targeted replacement — only the stat cards section changes, not the entire page.

Add import:
```typescript
import StatCard from '@/components/lattice/StatCard.vue'
```

Replace the hand-coded stat card template (the `<div v-for="(stat, idx) in stats" ...>` block and its children) with:

```vue
<div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
  <StatCard
    v-for="(stat, idx) in stats"
    :key="idx"
    :icon="iconByIndex[idx]"
    :label="t(titleKeyByIndex[idx])"
    :value="stat.value"
    :color="colorKeyByIndex[idx]"
  />
</div>
```

Add `colorKeyByIndex` array:
```typescript
const colorKeyByIndex: Array<'indigo' | 'emerald' | 'amber' | 'violet' | 'cyan'> = [
  'indigo', 'emerald', 'indigo', 'amber', 'violet',
]
```

Remove the old `colorByIndex` array and unused icon imports for stat card containers.

- [ ] **Step 2: Add lattice-card to Sandbox agent list cards**

In `fronted/src/pages/sandbox/index.vue`, find the agent card container div and add the `lattice-card` class. The specific class to add depends on the template — look for the outermost card wrapper div for each sandbox agent row/item and add `lattice-card p-6` or replace existing card classes.

- [ ] **Step 3: Add lattice-badge classes to AgentDetailDrawer status tags**

In `fronted/src/pages/sandbox/AgentDetailDrawer.vue`, find span tags that show tool call status (`ok`, `error`, `blocked`) and replace their inline color classes with:

```vue
<span v-if="t.status === 'ok'" class="lattice-badge-stable">ok</span>
<span v-else-if="t.status === 'error'" class="lattice-badge-roadmap">error</span>
<span v-else class="lattice-badge-pro">blocked</span>
```

- [ ] **Step 4: Add lattice-badge to Nodes page peer status**

In `fronted/src/pages/manage/nodes/index.vue`, find the peer/node status badges and replace with `lattice-badge-stable` for online/healthy peers.

- [ ] **Step 5: Full build verification**

```bash
cd fronted && pnpm build 2>&1 | tail -10
```

Expected: build success, all pages compile.

- [ ] **Step 6: Commit**

```bash
git add fronted/src/pages/dashboard/index.vue fronted/src/pages/sandbox/index.vue fronted/src/pages/sandbox/AgentDetailDrawer.vue fronted/src/pages/manage/nodes/index.vue
git commit -m "feat(dashboard): apply StatCard, lattice-card, lattice-badge classes across dashboard pages"
```

---

### Task 10: Final Verification

- [ ] **Step 1: Full build**

```bash
cd fronted && pnpm build 2>&1
```

Expected: zero errors.

- [ ] **Step 2: Verify git status is clean**

```bash
git status
```

Expected: nothing uncommitted.

- [ ] **Step 3: Quick visual smoke test**

```bash
cd fronted && pnpm dev
```

Open `http://localhost:5173/` — verify:
- Hero section has TopologyCanvas background animation (moving nodes and lines)
- Terminal has macOS-style chrome (red/yellow/green dots) and colored text
- Two Pillars section shows side-by-side cards with indigo/cyan accent
- Features grid has 3×2 cards with stable/PRO badges
- Quickstart has 3 terminal-style command blocks
- Pricing shows Community + PRO with indigo primary ring on PRO card
- All CTA buttons use indigo gradient
- Dashboard stat cards use the StatCard component with monospace numbers

- [ ] **Step 4: Commit any final tweaks**

```bash
git add -A
git commit -m "chore: final verification tweaks for frontend redesign"
```
