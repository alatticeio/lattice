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
          <span class="text-[10px] font-bold px-1.5 py-0.5 rounded-md bg-primary/10 text-primary ring-1 ring-primary/20">v0.1.2</span>
        </div>

        <nav class="hidden md:flex items-center gap-6 text-sm text-muted-foreground">
          <a href="#pillars"     class="hover:text-foreground transition-colors">{{ t('landing.nav.pillars') }}</a>
          <a href="#features"    class="hover:text-foreground transition-colors">{{ t('landing.nav.features') }}</a>
          <a href="#quickstart"  class="hover:text-foreground transition-colors">{{ t('landing.nav.quickstart') }}</a>
          <a href="#pricing"     class="hover:text-foreground transition-colors">{{ t('landing.nav.pricing') }}</a>
        </nav>

        <div class="flex items-center gap-2">
          <a href="https://github.com/alatticeio/lattice" target="_blank" rel="noopener noreferrer" aria-label="GitHub"
            class="text-muted-foreground hover:text-foreground transition-colors p-1.5 rounded-md hover:bg-muted">
            <svg class="size-4" viewBox="0 0 98 96" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
              <path fill-rule="evenodd" clip-rule="evenodd" d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z"/>
            </svg>
          </a>

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
              docker run -d --name lattice-k3s -p 443:443 ghcr.io/alatticeio/lattice-k3s:latest
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

    <!-- ── Pricing ───────────────────────────────────────────────── -->
    <section id="pricing" class="py-20 px-6">
      <div class="max-w-4xl mx-auto">
        <div class="text-center mb-12">
          <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-2">{{ t('landing.pricing.label') }}</p>
          <h2 class="text-2xl font-black tracking-tighter text-foreground">{{ t('landing.pricing.title') }}</h2>
          <p class="text-muted-foreground text-sm mt-2.5 max-w-md mx-auto leading-relaxed">
            {{ t('landing.pricing.subtitle') }}
          </p>
        </div>

        <div class="grid md:grid-cols-2 gap-5">
          <!-- Community -->
          <div class="lattice-card p-8 flex flex-col">
            <div class="mb-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-3">{{ t('landing.pricing.community_name') }}</p>
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
              <Button variant="outline" class="w-full border-border" size="lg">
                {{ t('landing.pricing.community_cta') }}
              </Button>
            </a>
          </div>

          <!-- Pro -->
          <div class="relative lattice-card ring-2 ring-primary p-8 flex flex-col shadow-lg shadow-primary/10">
            <!-- Badge -->
            <div class="absolute -top-3.5 left-1/2 -translate-x-1/2">
              <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-gradient-to-r from-indigo-600 to-indigo-500 text-white text-[11px] font-bold shadow-sm">
                <Crown class="size-3" /> {{ t('landing.pricing.pro_badge') }}
              </span>
            </div>

            <div class="mb-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-3">{{ t('landing.pricing.pro_name') }}</p>
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

            <Button
              class="w-full gap-2 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white border-0 shadow-md shadow-indigo-500/20"
              size="lg"
              @click="router.push('/auth/login')"
            >
              <Crown class="size-4" /> {{ t('landing.pricing.pro_cta') }}
            </Button>
            <p class="text-center text-[11px] text-muted-foreground mt-3">{{ t('landing.pricing.pro_disclaimer') }}</p>
          </div>
        </div>

        <!-- Enterprise hint -->
        <div class="mt-5 flex items-center justify-center gap-2 text-sm text-muted-foreground">
          <span>{{ t('landing.pricing.enterprise_text') }}</span>
          <a href="mailto:hello@alattice.io" class="text-foreground font-medium hover:underline underline-offset-4 transition-colors">{{ t('landing.pricing.enterprise_link') }}</a>
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
          <a href="#" class="hover:text-foreground transition-colors">{{ t('landing.nav.docs') }}</a>
          <a href="#pricing" class="hover:text-foreground transition-colors">{{ t('landing.nav.pricing') }}</a>
          <a href="https://github.com/alatticeio/lattice" target="_blank" rel="noopener noreferrer" class="hover:text-foreground transition-colors">{{ t('landing.nav.github') }}</a>
          <a href="/legal/privacy" class="text-xs text-muted-foreground hover:text-foreground transition-colors">隐私政策</a>
          <a href="/legal/terms" class="text-xs text-muted-foreground hover:text-foreground transition-colors">服务条款</a>
        </div>
      </div>
    </footer>

  </div>
</template>
