<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  ArrowRight, Network, Shield, Cpu, Layers, Zap, Globe,
  CheckCircle, ChevronRight, Terminal, Lock, LogOut, LayoutDashboard,
  Crown, X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { storeToRefs } from 'pinia'
import { useUserStore } from '@/stores/user'

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

const latency = ref(42)
const lastSync = ref('')
let timer: ReturnType<typeof setInterval>

onMounted(() => {
  lastSync.value = new Date().toLocaleTimeString([], { hour12: false })
  timer = setInterval(() => {
    latency.value = Math.floor(Math.random() * 8) + 38
    lastSync.value = new Date().toLocaleTimeString([], { hour12: false })
  }, 3000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="min-h-screen bg-background text-foreground antialiased">

    <!-- ── Navbar ─────────────────────────────────────────────────── -->
    <header class="sticky top-0 z-50 border-b border-border bg-background/80 backdrop-blur-md">
      <div class="max-w-6xl mx-auto px-6 h-14 flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <div class="size-7 rounded-lg bg-gradient-to-br from-indigo-600 to-cyan-500 flex items-center justify-center text-white text-xs font-black">
            L
          </div>
          <span class="font-black tracking-tighter text-sm">Lattice</span>
          <span class="text-[10px] font-bold px-1.5 py-0.5 rounded-md bg-primary/10 text-primary ring-1 ring-primary/20">v0.1.2</span>
        </div>

        <nav class="hidden md:flex items-center gap-6 text-sm text-muted-foreground">
          <a href="#features"      class="hover:text-foreground transition-colors">{{ t('landing.nav.features') }}</a>
          <a href="#architecture"  class="hover:text-foreground transition-colors">{{ t('landing.nav.architecture') }}</a>
          <a href="#pricing"       class="hover:text-foreground transition-colors">{{ t('landing.nav.pricing') }}</a>
          <a href="#quickstart"    class="hover:text-foreground transition-colors">{{ t('landing.nav.quickstart') }}</a>
        </nav>

        <div class="flex items-center gap-2">
          <a href="https://github.com/francisxys" target="_blank" rel="noopener noreferrer" aria-label="GitHub"
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
    <section class="relative overflow-hidden pt-24 pb-20 px-6">
      <!-- Subtle grid -->
      <div class="absolute inset-0 -z-10 [background-image:linear-gradient(to_right,rgba(0,0,0,.04)_1px,transparent_1px),linear-gradient(to_bottom,rgba(0,0,0,.04)_1px,transparent_1px)] dark:[background-image:linear-gradient(to_right,rgba(255,255,255,.04)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,.04)_1px,transparent_1px)] [background-size:48px_48px]" />
      <!-- Network topology SVG background -->
      <svg class="absolute inset-0 -z-10 w-full h-full text-indigo-500/10" viewBox="0 0 800 600" preserveAspectRatio="xMidYMid slice" fill="none" xmlns="http://www.w3.org/2000/svg">
        <line x1="150" y1="80" x2="350" y2="180" stroke="currentColor" stroke-width="1.5" />
        <line x1="350" y1="180" x2="550" y2="80" stroke="currentColor" stroke-width="1.5" />
        <line x1="350" y1="180" x2="250" y2="380" stroke="currentColor" stroke-width="1.5" />
        <line x1="350" y1="180" x2="450" y2="360" stroke="currentColor" stroke-width="1.5" />
        <line x1="150" y1="80" x2="80" y2="280" stroke="currentColor" stroke-width="1.5" />
        <line x1="550" y1="80" x2="650" y2="260" stroke="currentColor" stroke-width="1.5" />
        <line x1="80" y1="280" x2="250" y2="380" stroke="currentColor" stroke-width="1" />
        <line x1="450" y1="360" x2="650" y2="260" stroke="currentColor" stroke-width="1" />
        <circle cx="150" cy="80" r="5" fill="currentColor" />
        <circle cx="350" cy="180" r="7" class="fill-cyan-400/30" />
        <circle cx="550" cy="80" r="5" fill="currentColor" />
        <circle cx="80" cy="280" r="4" class="fill-cyan-400/30" />
        <circle cx="250" cy="380" r="4" fill="currentColor" />
        <circle cx="450" cy="360" r="4" class="fill-cyan-400/30" />
        <circle cx="650" cy="260" r="4" fill="currentColor" />
      </svg>
      <!-- Glow -->
      <div class="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-64 bg-indigo-500/10 rounded-full blur-3xl -z-10" />

      <div class="max-w-3xl mx-auto text-center relative">
        <div class="inline-flex items-center gap-2 px-3 py-1.5 mb-8 rounded-full border border-border bg-muted text-xs font-medium text-muted-foreground">
          <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
          {{ t('landing.hero.badge') }}
        </div>

        <h1 class="text-4xl md:text-[3.5rem] font-black tracking-tighter leading-[1.1] mb-5 bg-gradient-to-r from-gray-900 via-indigo-600 to-cyan-500 bg-clip-text text-transparent dark:from-gray-100 dark:via-indigo-400 dark:to-cyan-300">
          {{ t('landing.hero.title') }}
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

    <!-- ── Live stats terminal ────────────────────────────────────── -->
    <section class="px-6 pb-20">
      <div class="max-w-3xl mx-auto">
        <div class="rounded-2xl overflow-hidden border border-indigo-950 shadow-xl shadow-indigo-950/20 bg-gradient-to-b from-[#0f0d2e] to-[#1a1740]">
          <!-- Title bar -->
          <div class="flex items-center gap-1.5 px-4 py-2.5 bg-[#1e1b4b] border-b border-indigo-950">
            <div class="size-3 rounded-full bg-rose-500/70" />
            <div class="size-3 rounded-full bg-amber-400/70" />
            <div class="size-3 rounded-full bg-emerald-500/70" />
            <span class="ml-2 text-[11px] text-indigo-300/60 font-mono flex-1">lattice — control-plane</span>
            <div class="flex items-center gap-1.5">
              <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
              <span class="text-[11px] text-emerald-400 font-mono font-semibold">FABRIC ONLINE</span>
            </div>
          </div>
          <!-- Stats row -->
          <div class="grid grid-cols-3 divide-x divide-white/[0.06]">
            <div class="px-7 py-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-indigo-300/40 mb-2">{{ t('landing.stats.active_nodes') }}</p>
              <p class="text-3xl font-mono font-black text-white">128</p>
              <p class="text-[11px] text-emerald-400 font-semibold mt-1.5 flex items-center gap-1">
                <span class="size-1.5 rounded-full bg-emerald-500" /> {{ t('landing.stats.all_healthy') }}
              </p>
            </div>
            <div class="px-7 py-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-indigo-300/40 mb-2">{{ t('landing.stats.avg_latency') }}</p>
              <p class="text-3xl font-mono font-black bg-gradient-to-r from-indigo-400 to-cyan-400 bg-clip-text text-transparent transition-all duration-700">
                {{ latency }}<span class="text-lg text-indigo-300/40 ml-1">ms</span>
              </p>
              <p class="text-[11px] text-indigo-300/40 font-mono mt-1.5">{{ t('landing.stats.sync') }} {{ lastSync }}</p>
            </div>
            <div class="px-7 py-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-indigo-300/40 mb-2">{{ t('landing.stats.data_plane') }}</p>
              <p class="text-3xl font-mono font-black bg-gradient-to-r from-violet-400 to-cyan-400 bg-clip-text text-transparent italic">eBPF</p>
              <span class="inline-block mt-1.5 text-[10px] font-bold px-2 py-0.5 rounded bg-amber-400/10 text-amber-400 ring-1 ring-amber-400/20">{{ t('landing.features.tag_roadmap') }}</span>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ── Features ───────────────────────────────────────────────── -->
    <section id="features" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-5xl mx-auto">
        <div class="text-center mb-12">
          <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-2">{{ t('landing.features.label') }}</p>
          <h2 class="text-2xl font-black tracking-tighter text-foreground">{{ t('landing.features.title') }}</h2>
          <p class="text-muted-foreground text-sm mt-2.5 max-w-md mx-auto leading-relaxed">
            {{ t('landing.features.subtitle') }}
          </p>
        </div>

        <div class="grid md:grid-cols-3 gap-4">
          <!-- Card 1: CRDs -->
          <div class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 hover:-translate-y-0.5 transition-all duration-200">
            <div class="size-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center mb-4">
              <Layers class="size-5" />
            </div>
            <span class="text-[10px] font-bold px-2 py-0.5 rounded-full text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-500/10 ring-1 ring-emerald-200 dark:ring-emerald-500/20">{{ t('landing.features.tag_stable') }}</span>
            <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t('landing.features.card_1_title') }}</h3>
            <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.features.card_1_desc') }}</p>
          </div>

          <!-- Card 2: eBPF -->
          <div class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 hover:-translate-y-0.5 transition-all duration-200">
            <div class="size-10 rounded-xl bg-violet-50 dark:bg-violet-500/10 text-violet-600 dark:text-violet-400 flex items-center justify-center mb-4">
              <Cpu class="size-5" />
            </div>
            <span class="text-[10px] font-bold px-2 py-0.5 rounded-full text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-500/10 ring-1 ring-amber-200 dark:ring-amber-400/20">{{ t('landing.features.tag_roadmap') }}</span>
            <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t('landing.features.card_2_title') }}</h3>
            <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.features.card_2_desc') }}</p>
          </div>

          <!-- Card 3: Zero-Trust -->
          <div class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 hover:-translate-y-0.5 transition-all duration-200">
            <div class="size-10 rounded-xl bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center justify-center mb-4">
              <Lock class="size-5" />
            </div>
            <span class="text-[10px] font-bold px-2 py-0.5 rounded-full text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-500/10 ring-1 ring-emerald-200 dark:ring-emerald-500/20">{{ t('landing.features.tag_stable') }}</span>
            <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t('landing.features.card_3_title') }}</h3>
            <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.features.card_3_desc') }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- ── Advantages ─────────────────────────────────────────────── -->
    <section class="py-16 px-6">
      <div class="max-w-4xl mx-auto">
        <div class="grid grid-cols-2 md:grid-cols-3 gap-3">
          <div class="flex items-center gap-3 p-3.5 rounded-lg bg-muted border border-border hover:bg-muted/80 transition-colors">
            <Globe class="size-4 text-primary shrink-0" />
            <span class="text-sm text-foreground">{{ t('landing.advantages.item_1') }}</span>
          </div>
          <div class="flex items-center gap-3 p-3.5 rounded-lg bg-muted border border-border hover:bg-muted/80 transition-colors">
            <Zap class="size-4 text-primary shrink-0" />
            <span class="text-sm text-foreground">{{ t('landing.advantages.item_2') }}</span>
          </div>
          <div class="flex items-center gap-3 p-3.5 rounded-lg bg-muted border border-border hover:bg-muted/80 transition-colors">
            <Shield class="size-4 text-primary shrink-0" />
            <span class="text-sm text-foreground">{{ t('landing.advantages.item_3') }}</span>
          </div>
          <div class="flex items-center gap-3 p-3.5 rounded-lg bg-muted border border-border hover:bg-muted/80 transition-colors">
            <Layers class="size-4 text-primary shrink-0" />
            <span class="text-sm text-foreground">{{ t('landing.advantages.item_4') }}</span>
          </div>
          <div class="flex items-center gap-3 p-3.5 rounded-lg bg-muted border border-border hover:bg-muted/80 transition-colors">
            <Cpu class="size-4 text-primary shrink-0" />
            <span class="text-sm text-foreground">{{ t('landing.advantages.item_5') }}</span>
          </div>
          <div class="flex items-center gap-3 p-3.5 rounded-lg bg-muted border border-border hover:bg-muted/80 transition-colors">
            <Terminal class="size-4 text-primary shrink-0" />
            <span class="text-sm text-foreground">{{ t('landing.advantages.item_6') }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- ── IaC / Architecture ─────────────────────────────────────── -->
    <section id="architecture" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-5xl mx-auto">
        <div class="text-center mb-12">
          <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-2">{{ t('landing.architecture.label') }}</p>
          <h2 class="text-2xl font-black tracking-tighter text-foreground">{{ t('landing.architecture.title') }}</h2>
          <p class="text-muted-foreground text-sm mt-2.5 max-w-md mx-auto">
            {{ t('landing.architecture.subtitle') }}
          </p>
        </div>

        <div class="flex flex-col lg:flex-row gap-5">
          <!-- Steps -->
          <div class="lg:w-2/5 bg-card border border-border rounded-xl p-6">
            <div class="space-y-0">
              <!-- Step 1 -->
              <div class="flex items-start gap-3.5 relative">
                <div class="flex flex-col items-center">
                  <div class="size-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center text-[11px] font-black shrink-0">01</div>
                  <div class="w-px flex-1 bg-border min-h-[2.5rem]" />
                </div>
                <div class="pb-5">
                  <p class="text-sm font-semibold text-card-foreground">{{ t('landing.architecture.step_1_title') }}</p>
                  <p class="text-xs text-muted-foreground mt-0.5 leading-relaxed">{{ t('landing.architecture.step_1_desc') }}</p>
                </div>
              </div>
              <!-- Step 2 -->
              <div class="flex items-start gap-3.5 relative">
                <div class="flex flex-col items-center">
                  <div class="size-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center text-[11px] font-black shrink-0">02</div>
                  <div class="w-px flex-1 bg-border min-h-[2.5rem]" />
                </div>
                <div class="pb-5">
                  <p class="text-sm font-semibold text-card-foreground">{{ t('landing.architecture.step_2_title') }}</p>
                  <p class="text-xs text-muted-foreground mt-0.5 leading-relaxed">{{ t('landing.architecture.step_2_desc') }}</p>
                </div>
              </div>
              <!-- Step 3 -->
              <div class="flex items-start gap-3.5 relative">
                <div class="flex flex-col items-center">
                  <div class="size-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center text-[11px] font-black shrink-0">03</div>
                </div>
                <div>
                  <p class="text-sm font-semibold text-card-foreground">{{ t('landing.architecture.step_3_title') }}</p>
                  <p class="text-xs text-muted-foreground mt-0.5 leading-relaxed">{{ t('landing.architecture.step_3_desc') }}</p>
                </div>
              </div>
            </div>
          </div>

          <!-- Terminal -->
          <div class="lg:w-3/5 rounded-xl overflow-hidden border border-[#1e1b4b] bg-[#0f0d2e]">
            <div class="flex items-center gap-1.5 px-4 py-2.5 bg-[#1e1b4b] border-b border-[#1e1b4b]">
              <div class="size-2.5 rounded-full bg-rose-500/70" />
              <div class="size-2.5 rounded-full bg-amber-400/70" />
              <div class="size-2.5 rounded-full bg-emerald-500/70" />
              <span class="ml-2 text-[11px] text-indigo-300/60 font-mono">bash</span>
            </div>
            <div class="p-5 font-mono text-sm leading-7">
              <p><span class="text-indigo-300/30 select-none">#  </span><span class="text-indigo-300/50 italic">{{ t('landing.architecture.terminal_comment') }}</span></p>
              <p><span class="text-indigo-300/50 select-none">$  </span><span class="text-emerald-400">curl -sSL https://get.lattice.run \</span></p>
              <p><span class="text-indigo-300/20 select-none">   </span><span class="text-emerald-400">  | sudo bash -s -- join \</span></p>
              <p><span class="text-indigo-300/20 select-none">   </span><span class="text-emerald-400">  --token <span class="text-sky-400">wf_live_8s2k...92nz</span></span></p>
              <p class="mt-2"><span class="text-emerald-400/60 select-none">✓  </span><span class="text-emerald-500">{{ t('landing.architecture.terminal_success') }}</span></p>
            </div>
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
          <div class="bg-card border border-border rounded-2xl p-8 flex flex-col">
            <div class="mb-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-3">{{ t('landing.pricing.community_name') }}</p>
              <div class="flex items-end gap-1.5 mb-2">
                <span class="text-4xl font-black tracking-tighter text-foreground">{{ t('landing.pricing.community_price') }}</span>
              </div>
              <p class="text-xs text-muted-foreground">{{ t('landing.pricing.community_desc') }}</p>
            </div>

            <ul class="space-y-3 mb-8 flex-1">
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.community_feat_1') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.community_feat_2') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.community_feat_3') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.community_feat_4') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.community_feat_5') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.community_feat_6') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.community_feat_7') }}
              </li>
              <!-- Locked pro features -->
              <li class="flex items-center gap-2.5 text-sm text-muted-foreground/50 line-through">
                <X class="size-4 text-muted-foreground/30 shrink-0" />
                {{ t('landing.pricing.pro_feat_locked_1') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-muted-foreground/50 line-through">
                <X class="size-4 text-muted-foreground/30 shrink-0" />
                {{ t('landing.pricing.pro_feat_locked_2') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-muted-foreground/50 line-through">
                <X class="size-4 text-muted-foreground/30 shrink-0" />
                {{ t('landing.pricing.pro_feat_locked_3') }}
              </li>
            </ul>

            <a href="https://github.com/francisxys" target="_blank" rel="noopener noreferrer">
              <Button variant="outline" class="w-full border-border" size="lg">
                {{ t('landing.pricing.community_cta') }}
              </Button>
            </a>
          </div>

          <!-- Pro -->
          <div class="relative bg-card border-2 border-primary rounded-2xl p-8 flex flex-col shadow-lg shadow-primary/10">
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
              <li class="flex items-center gap-2.5 text-sm font-medium text-primary">
                <CheckCircle class="size-4 shrink-0" />
                {{ t('landing.pricing.pro_feat_all') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.pro_feat_1') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.pro_feat_2') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.pro_feat_3') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.pro_feat_4') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.pro_feat_5') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.pro_feat_6') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.pro_feat_7') }}
              </li>
              <li class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ t('landing.pricing.pro_feat_8') }}
              </li>
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
          <a href="mailto:hello@lattice.run" class="text-foreground font-medium hover:underline underline-offset-4 transition-colors">{{ t('landing.pricing.enterprise_link') }}</a>
        </div>
      </div>
    </section>

    <!-- ── CTA ────────────────────────────────────────────────────── -->
    <section id="quickstart" class="py-20 px-6">
      <div class="max-w-xl mx-auto text-center">
        <div class="size-14 rounded-2xl bg-gradient-to-br from-indigo-600/10 to-cyan-500/10 flex items-center justify-center mx-auto mb-5">
          <Network class="size-7 text-indigo-500" />
        </div>
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

        <div class="grid grid-cols-3 gap-2 text-left max-w-xs mx-auto">
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <CheckCircle class="size-3.5 text-emerald-500 shrink-0" />
            {{ t('landing.cta.badge_1') }}
          </div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <CheckCircle class="size-3.5 text-emerald-500 shrink-0" />
            {{ t('landing.cta.badge_2') }}
          </div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <CheckCircle class="size-3.5 text-emerald-500 shrink-0" />
            {{ t('landing.cta.badge_3') }}
          </div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <CheckCircle class="size-3.5 text-emerald-500 shrink-0" />
            {{ t('landing.cta.badge_4') }}
          </div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <CheckCircle class="size-3.5 text-emerald-500 shrink-0" />
            {{ t('landing.cta.badge_5') }}
          </div>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <CheckCircle class="size-3.5 text-emerald-500 shrink-0" />
            {{ t('landing.cta.badge_6') }}
          </div>
        </div>
      </div>
    </section>

    <!-- ── Footer ─────────────────────────────────────────────────── -->
    <footer class="border-t border-border px-6 py-7">
      <div class="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
        <div class="flex items-center gap-2">
          <div class="size-5 rounded bg-gradient-to-br from-indigo-600 to-cyan-500 flex items-center justify-center text-white text-[10px] font-black">
            L
          </div>
          <span class="text-sm font-black tracking-tighter text-muted-foreground">Lattice</span>
        </div>
        <p class="text-[11px] text-muted-foreground font-mono uppercase tracking-widest">
          {{ t('landing.footer.copyright') }}
        </p>
        <div class="flex items-center gap-5 text-xs text-muted-foreground">
          <a href="#" class="hover:text-foreground transition-colors">{{ t('landing.nav.docs') }}</a>
          <a href="#pricing" class="hover:text-foreground transition-colors">{{ t('landing.nav.pricing') }}</a>
          <a href="https://github.com/francisxys" target="_blank" rel="noopener noreferrer" class="hover:text-foreground transition-colors">{{ t('landing.nav.github') }}</a>
          <a href="#" class="hover:text-foreground transition-colors">{{ t('landing.nav.community') }}</a>
        </div>
      </div>
    </footer>

  </div>
</template>
