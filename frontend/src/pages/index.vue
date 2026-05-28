<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  ArrowRight, Zap, Globe, Shield,
  CheckCircle, ChevronRight, LogOut, LayoutDashboard,
  Network, Container, Cpu, Terminal, BookOpen, Code2, Workflow, Check, Tag,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import AvatarPreset from '@/components/AvatarPreset.vue'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { setLocale } from '@/locales'
import type { Locale } from '@/locales'
import { storeToRefs } from 'pinia'
import { useUserStore } from '@/stores/user'
import LatticeTerminal from '@/components/lattice/LatticeTerminal.vue'
import TopologyCanvas from '@/components/lattice/TopologyCanvas.vue'
import SectionHeader from '@/components/lattice/SectionHeader.vue'
import type { TerminalLine } from '@/components/lattice/LatticeTerminal.vue'
import DemoModal from '@/components/DemoModal.vue'
import SandboxDemoModal from '@/components/SandboxDemoModal.vue'

definePage({ meta: { layout: 'blank' } })

const { t, locale } = useI18n()
const router = useRouter()

function switchLocale(lang: Locale) {
  setLocale(lang)
}

const localeOptions: { value: Locale; label: string }[] = [
  { value: 'zh-CN', label: '中文' },
  { value: 'en',    label: 'English' },
]
const userStore = useUserStore()
const { userInfo } = storeToRefs(userStore)
const { logout } = userStore

// ── Version tags ─────────────────────────────────────────────
const versions = ref<string[]>([])
const latestVersion = computed(() => versions.value[0] ?? 'v0.2.0')

// ── Demo ──────────────────────────────────────────────────────
const demoOpen = ref(false)
const sandboxDemoOpen = ref(false)
const demoEnabled = ref(false)

onMounted(async () => {
  try {
    const res = await fetch('https://api.github.com/repos/alatticeio/lattice/tags?per_page=10')
    if (res.ok) {
      const data: { name: string }[] = await res.json()
      versions.value = data.map(t => t.name)
    }
  } catch {
    versions.value = ['v0.2.0']
  }
  try {
    const r = await fetch('/api/v1/demo/status')
    if (r.ok) {
      const body = await r.json()
      demoEnabled.value = body?.data?.enabled === true
    }
  } catch { /* non-fatal */ }
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

const saifItems = [
  { key: 'item_1', icon: '🔒', iconBg: 'bg-blue-500/10 text-blue-600 dark:text-blue-400' },
  { key: 'item_2', icon: '🛡', iconBg: 'bg-green-500/10 text-green-600 dark:text-green-400' },
  { key: 'item_3', icon: '🎯', iconBg: 'bg-violet-500/10 text-violet-600 dark:text-violet-400' },
  { key: 'item_4', icon: '⚙', iconBg: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
]

const whyItems = [
  { key: 'item_1', icon: '🔑' },
  { key: 'item_2', icon: '📋' },
  { key: 'item_3', icon: '🚫' },
]

const ossStats = [
  { valueKey: 'stat_1_value', labelKey: 'stat_1_label' },
  { valueKey: 'stat_2_value', labelKey: 'stat_2_label' },
  { valueKey: 'stat_3_value', labelKey: 'stat_3_label' },
  { valueKey: 'stat_4_value', labelKey: 'stat_4_label' },
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
  return tag === 'stable' ? 'lattice-badge lattice-badge-stable'
    : tag === 'roadmap' ? 'lattice-badge lattice-badge-roadmap'
    : 'lattice-badge lattice-badge-pro'
}

function tagLabel(tag: string) {
  if (tag === 'stable') return t('landing.features.tag_stable')
  if (tag === 'roadmap') return t('landing.features.tag_roadmap')
  return 'PRO'
}
</script>

<template>
  <div class="min-h-screen bg-background text-foreground antialiased">

    <!-- ── Navbar ─────────────────────────────────────────────────── -->
    <header class="sticky top-0 z-50 border-b border-border bg-background/80 backdrop-blur-md">
      <div class="max-w-6xl mx-auto px-6 h-14 flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <img src="@/assets/logo.svg" class="size-7" alt="Lattice" />
          <span class="font-black tracking-tighter text-sm">Lattice</span>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <button class="flex items-center gap-0.5 text-[10px] font-bold px-1.5 py-0.5 rounded-md bg-primary/10 text-primary ring-1 ring-primary/20 hover:bg-primary/20 transition-colors">
                {{ latestVersion }}
                <ChevronRight class="size-2.5 rotate-90 opacity-60" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" class="min-w-[120px]">
              <DropdownMenuItem
                v-for="v in versions"
                :key="v"
                as-child
              >
                <a
                  :href="`https://github.com/alatticeio/lattice/releases/tag/${v}`"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="flex items-center gap-2 text-xs"
                >
                  <Tag class="size-3 text-muted-foreground" />
                  {{ v }}
                </a>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <nav class="hidden md:flex items-center gap-6 text-sm text-muted-foreground">
          <a href="#pillars"     class="hover:text-foreground transition-colors">{{ t('landing.nav.pillars') }}</a>
          <a href="#features"    class="hover:text-foreground transition-colors">{{ t('landing.nav.features') }}</a>
          <a href="#quickstart"  class="hover:text-foreground transition-colors">{{ t('landing.nav.quickstart') }}</a>
        </nav>

        <div class="flex items-center gap-2">
          <a href="https://github.com/alatticeio/lattice" target="_blank" rel="noopener noreferrer" aria-label="GitHub"
            class="text-muted-foreground hover:text-foreground transition-colors p-1.5 rounded-md hover:bg-muted">
            <svg class="size-4" viewBox="0 0 98 96" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
              <path fill-rule="evenodd" clip-rule="evenodd" d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z"/>
            </svg>
          </a>

          <!-- Language switcher -->
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <button class="text-muted-foreground hover:text-foreground hover:bg-muted rounded-lg px-2 py-1.5 text-xs font-medium transition-colors">
                {{ locale === 'zh-CN' ? '中文' : 'EN' }}
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="min-w-32">
              <DropdownMenuItem
                v-for="opt in localeOptions"
                :key="opt.value"
                @click="switchLocale(opt.value)"
                class="flex items-center justify-between"
              >
                {{ opt.label }}
                <Check v-if="locale === opt.value" class="size-3.5 text-primary" />
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <!-- 未登录：显示登录按钮 -->
          <template v-if="!userInfo">
            <Button variant="ghost" size="sm" class="text-muted-foreground" @click="router.push('/auth/login')">{{ t('landing.nav.login') }}</Button>
            <Button size="sm" class="gap-1.5 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white border-0" @click="router.push('/dashboard')">
              {{ t('landing.nav.console') }} <ArrowRight class="size-3.5" />
            </Button>
          </template>

          <!-- 已登录：显示用户头像下拉菜单 -->
          <template v-else>
            <Button size="sm" class="gap-1.5 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white border-0" @click="router.push('/dashboard')">
              {{ t('landing.nav.console') }} <ArrowRight class="size-3.5" />
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <button class="hover:ring-border flex items-center gap-2 rounded-lg px-1.5 py-1 transition-colors hover:ring-2 hover:bg-muted">
                  <AvatarPreset :avatar-url="userInfo.avatarUrl ?? ''" :size="28" :fallback="userInfo.username" />
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

    <!-- ── Hero ───────────────────────────────────────────────────── -->
    <section class="relative overflow-hidden pt-28 pb-24 px-6">
      <!-- Topology background -->
      <div class="absolute inset-0 -z-10 opacity-40">
        <TopologyCanvas :node-count="14" />
      </div>
      <!-- Glow -->
      <div class="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-64 bg-indigo-500/10 rounded-full blur-3xl -z-10" />

      <div class="max-w-3xl mx-auto text-center relative">
        <div class="inline-flex items-center gap-2 px-3 py-1.5 mb-8 rounded-full border border-border bg-muted text-xs font-medium text-muted-foreground">
          <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
          {{ t('landing.hero.badge') }}
        </div>

        <h1 class="text-4xl md:text-[3.5rem] font-black tracking-tighter leading-[1.1] mb-5">
          <span class="lattice-gradient-text block">{{ t('landing.hero.title_line1') }}</span>
          <span class="text-foreground">{{ t('landing.hero.title_line2') }}</span>
        </h1>

        <p class="text-muted-foreground text-base leading-relaxed max-w-xl mx-auto mb-8">
          {{ t('landing.hero.subtitle') }}
        </p>

        <div class="flex flex-col sm:flex-row items-center justify-center gap-3">
          <Button
            size="lg"
            class="gap-2 px-7 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white border-0 shadow-lg shadow-indigo-500/20"
            @click="router.push('/manage/stepper')"
          >
            <Zap class="size-4" /> {{ t('landing.hero.cta_primary') }}
          </Button>
          <Button
            variant="outline"
            size="lg"
            class="gap-2 px-7 border-border"
            @click="router.push('/dashboard')"
          >
            {{ t('landing.hero.cta_secondary') }} <ChevronRight class="size-4" />
          </Button>
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
        </div>
      </div>
    </section>

    <!-- ── Terminal Demo ──────────────────────────────────────────── -->
    <section class="px-6 pb-20">
      <div class="max-w-3xl mx-auto">
        <LatticeTerminal
          :title="t('landing.terminal.title')"
          :status="t('landing.terminal.status')"
          :lines="terminalLines"
        />
        <p class="text-center text-xs text-muted-foreground font-mono mt-4">
          {{ t('landing.terminal.caption') }}
        </p>
      </div>
    </section>

    <!-- ── The Problem ───────────────────────────────────────────── -->
    <section id="problem" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-4xl mx-auto">
        <SectionHeader
          :tag="t('landing.problem.tag')"
          :title="t('landing.problem.title')"
          :subtitle="t('landing.problem.subtitle')"
          subtitleClass="max-w-2xl"
        />

        <div class="space-y-3 mb-8">
          <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground text-center mb-5">
            {{ t('landing.problem.saif_label') }}
          </p>
          <div
            v-for="item in saifItems"
            :key="item.key"
            class="flex items-start gap-4 p-4 rounded-xl border border-border bg-background"
          >
            <div :class="[item.iconBg, 'size-10 rounded-lg flex items-center justify-center shrink-0 text-xl']">
              {{ item.icon }}
            </div>
            <div>
              <p class="text-sm font-bold text-foreground">
                {{ t('landing.problem.' + item.key + '_title') }}
                <span class="text-xs font-normal text-muted-foreground ml-1.5">
                  {{ t('landing.problem.' + item.key + '_sub') }}
                </span>
              </p>
              <p class="text-xs text-muted-foreground mt-1 leading-relaxed">
                {{ t('landing.problem.' + item.key + '_desc') }}
              </p>
            </div>
          </div>
        </div>

        <p class="text-center text-xs text-muted-foreground">
          {{ t('landing.problem.ref_prefix') }}<a
            href="https://safety.google/cybersecurity-advancements/saif/"
            target="_blank"
            rel="noopener noreferrer"
            class="text-primary hover:underline underline-offset-4"
          >{{ t('landing.problem.ref_link') }}</a>{{ t('landing.problem.ref_suffix') }}
        </p>
      </div>
    </section>

    <!-- ── Why Lattice ───────────────────────────────────────────── -->
    <section id="why" class="py-20 px-6">
      <div class="max-w-4xl mx-auto">
        <SectionHeader
          :tag="t('landing.why.tag')"
          :title="t('landing.why.title')"
          :subtitle="t('landing.why.subtitle')"
          subtitleClass="max-w-2xl"
        />

        <div class="grid grid-cols-1 md:grid-cols-3 gap-5">
          <div
            v-for="item in whyItems"
            :key="item.key"
            class="p-6 rounded-xl border border-border bg-muted/50"
          >
            <div class="text-2xl mb-4">{{ item.icon }}</div>
            <p class="text-sm font-bold text-foreground mb-2">{{ t('landing.why.' + item.key + '_title') }}</p>
            <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.why.' + item.key + '_desc') }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- ── Two Pillars ────────────────────────────────────────────── -->
    <section id="pillars" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-5xl mx-auto">
        <SectionHeader
          :tag="t('landing.pillars.tag')"
          :title="t('landing.pillars.title')"
          :subtitle="t('landing.pillars.subtitle')"
        />

        <div class="grid md:grid-cols-2 gap-5">
          <!-- Network Pillar -->
          <div class="lattice-card p-8">
            <div class="size-11 rounded-xl bg-primary/10 text-primary flex items-center justify-center mb-5">
              <Globe class="size-6" />
            </div>
            <h3 class="text-lg font-bold mb-5 text-card-foreground">{{ t('landing.pillars.net_title') }}</h3>
            <div class="space-y-5">
              <div v-for="(item, i) in pillarNetwork" :key="'net-' + i" class="flex items-start gap-3.5">
                <div class="size-9 rounded-lg bg-muted text-muted-foreground flex items-center justify-center shrink-0 mt-0.5">
                  <component :is="item.icon" class="size-4" />
                </div>
                <div>
                  <p class="text-sm font-semibold text-card-foreground">{{ t(item.titleKey) }}</p>
                  <p class="text-xs text-muted-foreground mt-0.5 leading-relaxed">{{ t(item.descKey) }}</p>
                </div>
              </div>
            </div>
          </div>

          <!-- Agent Pillar -->
          <div class="lattice-card p-8">
            <div class="size-11 rounded-xl bg-primary/10 text-primary flex items-center justify-center mb-5">
              <Container class="size-6" />
            </div>
            <h3 class="text-lg font-bold mb-5 text-card-foreground">{{ t('landing.pillars.agent_title') }}</h3>
            <div class="space-y-5">
              <div v-for="(item, i) in pillarAgent" :key="'agent-' + i" class="flex items-start gap-3.5">
                <div class="size-9 rounded-lg bg-muted text-muted-foreground flex items-center justify-center shrink-0 mt-0.5">
                  <component :is="item.icon" class="size-4" />
                </div>
                <div>
                  <p class="text-sm font-semibold text-card-foreground">{{ t(item.titleKey) }}</p>
                  <p class="text-xs text-muted-foreground mt-0.5 leading-relaxed">{{ t(item.descKey) }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ── Features Grid ──────────────────────────────────────────── -->
    <section id="features" class="py-20 px-6">
      <div class="max-w-5xl mx-auto">
        <SectionHeader
          :tag="t('landing.features.label')"
          :title="t('landing.features.title')"
          :subtitle="t('landing.features.subtitle')"
        />

        <div class="grid md:grid-cols-3 gap-4">
          <div v-for="(feat, i) in features" :key="i" class="lattice-card p-6">
            <div class="size-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center mb-4">
              <component :is="feat.icon" class="size-5" />
            </div>
            <span :class="tagClass(feat.tag)">{{ tagLabel(feat.tag) }}</span>
            <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t(feat.titleKey) }}</h3>
            <p class="text-xs text-muted-foreground leading-relaxed">{{ t(feat.descKey) }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- ── Quickstart ─────────────────────────────────────────────── -->
    <section id="quickstart" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-3xl mx-auto">
        <SectionHeader
          :tag="t('landing.quickstart.tag')"
          :title="t('landing.quickstart.title')"
          :subtitle="t('landing.quickstart.subtitle')"
        />

        <div class="space-y-3">
          <!-- Docker -->
          <div class="lattice-card p-5">
            <code class="text-sm font-mono text-card-foreground block mb-2">
              docker run -d --name lattice-k3s -p 8080:8080 ghcr.io/alatticeio/lattice-k3s:latest
            </code>
            <p class="text-xs text-muted-foreground">{{ t('landing.quickstart.docker_hint') }}</p>
          </div>

          <!-- K8s -->
          <div class="lattice-card p-5">
            <code class="text-sm font-mono text-card-foreground block mb-2">
              kubectl apply -k https://github.com/alatticeio/lattice/config/lattice/overlays/all-in-one
            </code>
            <p class="text-xs text-muted-foreground">{{ t('landing.quickstart.k8s_hint') }}</p>
          </div>

          <!-- Sandbox -->
          <div class="lattice-card p-5">
            <code class="text-sm font-mono text-card-foreground block mb-2">
              lattice sandbox start --name my-agent --token lt-enroll-xxx
            </code>
            <p class="text-xs text-muted-foreground">{{ t('landing.quickstart.sandbox_hint') }}</p>
          </div>
        </div>
      </div>
    </section>


    <!-- ── Open Source ───────────────────────────────────────────── -->
    <section id="open-source" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-4xl mx-auto">
        <SectionHeader
          :tag="t('landing.oss.tag')"
          :title="t('landing.oss.title')"
          :subtitle="t('landing.oss.subtitle')"
          subtitleClass="max-w-2xl"
        />

        <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
          <div
            v-for="item in ossStats"
            :key="item.valueKey"
            class="text-center p-5 rounded-xl border border-border bg-background"
          >
            <p class="text-lg font-black tracking-tight text-foreground">{{ t('landing.oss.' + item.valueKey) }}</p>
            <p class="text-xs text-muted-foreground mt-1">{{ t('landing.oss.' + item.labelKey) }}</p>
          </div>
        </div>

        <div class="flex flex-col sm:flex-row items-center justify-center gap-3">
          <Button
            variant="outline"
            size="lg"
            class="gap-2 border-border"
            as="a"
            href="https://github.com/alatticeio/lattice"
            target="_blank"
            rel="noopener noreferrer"
          >
            <svg class="size-4" viewBox="0 0 98 96" xmlns="http://www.w3.org/2000/svg" fill="currentColor" aria-hidden="true">
              <path fill-rule="evenodd" clip-rule="evenodd" d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z"/>
            </svg>
            {{ t('landing.oss.cta') }}
          </Button>
          <span class="text-xs text-muted-foreground font-mono">{{ t('landing.oss.cta_sub') }}</span>
        </div>
      </div>
    </section>

    <!-- ── CTA ────────────────────────────────────────────────────── -->
    <section class="py-20 px-6">
      <div class="max-w-xl mx-auto text-center">
        <h2 class="text-2xl font-black tracking-tighter mb-3 text-foreground">{{ t('landing.cta.title') }}</h2>
        <p class="text-muted-foreground text-sm leading-relaxed mb-7 max-w-sm mx-auto">
          {{ t('landing.cta.subtitle') }}
        </p>
        <div class="flex flex-col sm:flex-row gap-3 justify-center mb-8">
          <Button
            size="lg"
            class="gap-2 px-8 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white border-0 shadow-lg shadow-indigo-500/20"
            @click="router.push('/manage/stepper')"
          >
            <Zap class="size-4" /> {{ t('landing.cta.button_primary') }}
          </Button>
          <Button
            variant="outline"
            size="lg"
            class="gap-2 px-8 border-border"
            @click="router.push('/dashboard')"
          >
            {{ t('landing.cta.button_secondary') }} <ArrowRight class="size-4" />
          </Button>
        </div>
        <div class="flex flex-wrap justify-center gap-x-4 gap-y-2 max-w-xs mx-auto">
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
        <p class="text-[11px] text-muted-foreground font-mono uppercase tracking-widest">
          {{ t('landing.footer.copyright') }}
        </p>
        <div class="flex items-center gap-5 text-xs text-muted-foreground">
          <a href="https://docs.alattice.io" target="_blank" rel="noopener noreferrer" class="hover:text-foreground transition-colors">{{ t('landing.nav.docs') }}</a>
          <a href="https://github.com/alatticeio/lattice" target="_blank" rel="noopener noreferrer" class="hover:text-foreground transition-colors">{{ t('landing.nav.github') }}</a>
          <a href="/legal/privacy" class="text-xs text-muted-foreground hover:text-foreground transition-colors">{{ t('landing.footer.privacy') }}</a>
          <a href="/legal/terms" class="text-xs text-muted-foreground hover:text-foreground transition-colors">{{ t('landing.footer.terms') }}</a>
        </div>
      </div>
    </footer>

  </div>

  <DemoModal v-model:open="demoOpen" />
  <SandboxDemoModal v-model:open="sandboxDemoOpen" />
</template>
