<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
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
import { useUserStore } from '@/stores/user'

definePage({ meta: { layout: 'blank' } })

const router = useRouter()
const userStore = useUserStore()
const { userInfo, logout } = userStore

const avatarFallback = computed(() => {
  const name = userInfo?.username ?? userInfo?.email ?? '?'
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

const features = [
  {
    icon: Layers,
    tag: 'Stable',
    tagClass: 'text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-500/10 ring-1 ring-emerald-200 dark:ring-emerald-500/20',
    title: 'Operator-Native CRDs',
    desc: '深度集成 Kubernetes 生态，通过声明式 CRD 定义网络拓扑，网络即代码。',
    iconBg: 'bg-primary/10',
    iconColor: 'text-primary',
  },
  {
    icon: Cpu,
    tag: 'Roadmap',
    tagClass: 'text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-500/10 ring-1 ring-amber-200 dark:ring-amber-400/20',
    title: 'eBPF Acceleration',
    desc: '利用 eBPF 在内核态实现数据包卸载，绕过协议栈提升吞吐性能与可观测性。',
    iconBg: 'bg-violet-50 dark:bg-violet-500/10',
    iconColor: 'text-violet-600 dark:text-violet-400',
  },
  {
    icon: Lock,
    tag: 'Stable',
    tagClass: 'text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-500/10 ring-1 ring-emerald-200 dark:ring-emerald-500/20',
    title: 'Zero-Trust Overlay',
    desc: '基于身份的零信任验证建立 WireGuard 隧道，端到端加密，无需信任底层网络。',
    iconBg: 'bg-emerald-50 dark:bg-emerald-500/10',
    iconColor: 'text-emerald-600 dark:text-emerald-400',
  },
]

const advantages = [
  { icon: Globe,    text: '多云 / 边缘节点统一接入' },
  { icon: Zap,      text: '毫秒级拓扑同步与故障恢复' },
  { icon: Shield,   text: '端到端 WireGuard 加密' },
  { icon: Layers,   text: 'Kubernetes CRD 声明式驱动' },
  { icon: Cpu,      text: 'eBPF 内核态数据平面（路线图）' },
  { icon: Terminal, text: '一行命令完成节点接入' },
]
</script>

<template>
  <div class="min-h-screen bg-background text-foreground antialiased">

    <!-- ── Navbar ─────────────────────────────────────────────────── -->
    <header class="sticky top-0 z-50 border-b border-border bg-background/80 backdrop-blur-md">
      <div class="max-w-6xl mx-auto px-6 h-14 flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <img src="@/assets/logo.svg" class="size-7" alt="Wireflow" />
          <span class="font-black tracking-tighter text-sm">Wireflow</span>
          <span class="text-[10px] font-bold px-1.5 py-0.5 rounded-md bg-primary/10 text-primary ring-1 ring-primary/20">v0.1.2</span>
        </div>

        <nav class="hidden md:flex items-center gap-6 text-sm text-muted-foreground">
          <a href="#features"      class="hover:text-foreground transition-colors">功能特性</a>
          <a href="#architecture"  class="hover:text-foreground transition-colors">架构说明</a>
          <a href="#pricing"       class="hover:text-foreground transition-colors">定价</a>
          <a href="#quickstart"    class="hover:text-foreground transition-colors">快速接入</a>
        </nav>

        <div class="flex items-center gap-2">
          <a href="https://github.com/francisxys" target="_blank" rel="noopener noreferrer"
            class="text-muted-foreground hover:text-foreground transition-colors p-1.5 rounded-md hover:bg-muted">
            <svg class="size-4" viewBox="0 0 98 96" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
              <path fill-rule="evenodd" clip-rule="evenodd" d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z"/>
            </svg>
          </a>

          <!-- 未登录：显示登录按钮 -->
          <template v-if="!userInfo">
            <Button variant="ghost" size="sm" class="text-muted-foreground" @click="router.push('/auth/login')">登录</Button>
            <Button size="sm" class="gap-1.5 bg-primary hover:bg-primary/90 text-primary-foreground border-0" @click="router.push('/dashboard')">
              进入控制台 <ArrowRight class="size-3.5" />
            </Button>
          </template>

          <!-- 已登录：显示用户头像下拉菜单 -->
          <template v-else>
            <Button size="sm" class="gap-1.5 bg-primary hover:bg-primary/90 text-primary-foreground border-0" @click="router.push('/dashboard')">
              进入控制台 <ArrowRight class="size-3.5" />
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
                  <span>控制台</span>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem class="text-destructive focus:text-destructive" @click="logout()">
                  <LogOut class="mr-2 size-4" />
                  <span>退出登录</span>
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
      <!-- Glow -->
      <div class="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-64 bg-primary/10 dark:bg-primary/5 rounded-full blur-3xl -z-10" />

      <div class="max-w-3xl mx-auto text-center">
        <div class="inline-flex items-center gap-2 px-3 py-1.5 mb-8 rounded-full border border-border bg-muted text-xs font-medium text-muted-foreground">
          <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
          WIREFLOW ENGINE · 自动化网络编排平台
        </div>

        <h1 class="text-4xl md:text-[3.5rem] font-black tracking-tighter leading-[1.1] mb-5">
          构建声明式<br />
          <span class="text-primary">云原生智能组网</span>
        </h1>

        <p class="text-muted-foreground text-base leading-relaxed max-w-xl mx-auto mb-8">
          基于 <span class="text-foreground font-medium">零信任架构</span> 与
          <span class="text-foreground font-medium">Kubernetes 原生驱动</span>，
          为异构多云及边缘环境提供高性能、透明且可观测的 Mesh 网络层。
        </p>

        <div class="flex flex-col sm:flex-row items-center justify-center gap-3">
          <Button
            size="lg"
            class="gap-2 px-7 bg-primary hover:bg-primary/90 text-primary-foreground border-0 shadow-lg shadow-primary/20"
            @click="router.push('/manage/stepper')"
          >
            <Zap class="size-4" /> 快速接入节点
          </Button>
          <Button
            variant="outline"
            size="lg"
            class="gap-2 px-7 border-border"
            @click="router.push('/dashboard')"
          >
            进入控制台 <ChevronRight class="size-4" />
          </Button>
        </div>
      </div>
    </section>

    <!-- ── Live stats terminal ────────────────────────────────────── -->
    <section class="px-6 pb-20">
      <div class="max-w-3xl mx-auto">
        <div class="rounded-2xl overflow-hidden border border-zinc-800 shadow-xl shadow-zinc-900/20">
          <!-- Title bar -->
          <div class="flex items-center gap-1.5 px-4 py-2.5 bg-zinc-900 border-b border-zinc-800">
            <div class="size-3 rounded-full bg-rose-500/70" />
            <div class="size-3 rounded-full bg-amber-400/70" />
            <div class="size-3 rounded-full bg-emerald-500/70" />
            <span class="ml-2 text-[11px] text-zinc-500 font-mono flex-1">wireflow — control-plane</span>
            <div class="flex items-center gap-1.5">
              <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
              <span class="text-[11px] text-emerald-400 font-mono font-semibold">FABRIC ONLINE</span>
            </div>
          </div>
          <!-- Stats row -->
          <div class="grid grid-cols-3 divide-x divide-zinc-800 bg-zinc-950">
            <div class="px-7 py-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-zinc-600 mb-2">Active Nodes</p>
              <p class="text-3xl font-mono font-black text-zinc-100">128</p>
              <p class="text-[11px] text-emerald-400 font-semibold mt-1.5 flex items-center gap-1">
                <span class="size-1.5 rounded-full bg-emerald-500" /> All Healthy
              </p>
            </div>
            <div class="px-7 py-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-zinc-600 mb-2">Avg Latency</p>
              <p class="text-3xl font-mono font-black text-indigo-400 transition-all duration-700">
                {{ latency }}<span class="text-lg text-zinc-600 ml-1">ms</span>
              </p>
              <p class="text-[11px] text-zinc-600 font-mono mt-1.5">sync {{ lastSync }}</p>
            </div>
            <div class="px-7 py-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-zinc-600 mb-2">Data Plane</p>
              <p class="text-3xl font-mono font-black text-violet-400 italic">eBPF</p>
              <span class="inline-block mt-1.5 text-[10px] font-bold px-2 py-0.5 rounded bg-amber-400/10 text-amber-400 ring-1 ring-amber-400/20">Roadmap</span>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ── Features ───────────────────────────────────────────────── -->
    <section id="features" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-5xl mx-auto">
        <div class="text-center mb-12">
          <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-2">Core Features</p>
          <h2 class="text-2xl font-black tracking-tighter text-foreground">为云原生时代而生的组网平台</h2>
          <p class="text-muted-foreground text-sm mt-2.5 max-w-md mx-auto leading-relaxed">
            将 WireGuard 的极简高效与 Kubernetes 声明式能力融合，打造下一代自动化网络编排基础设施。
          </p>
        </div>

        <div class="grid md:grid-cols-3 gap-4">
          <div
            v-for="feat in features" :key="feat.title"
            class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 transition-all duration-200"
          >
            <div class="size-10 rounded-lg flex items-center justify-center mb-4" :class="[feat.iconBg, feat.iconColor]">
              <component :is="feat.icon" class="size-5" />
            </div>
            <span class="text-[10px] font-bold px-2 py-0.5 rounded-full" :class="feat.tagClass">{{ feat.tag }}</span>
            <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ feat.title }}</h3>
            <p class="text-xs text-muted-foreground leading-relaxed">{{ feat.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- ── Advantages ─────────────────────────────────────────────── -->
    <section class="py-16 px-6">
      <div class="max-w-4xl mx-auto">
        <div class="grid grid-cols-2 md:grid-cols-3 gap-3">
          <div
            v-for="item in advantages" :key="item.text"
            class="flex items-center gap-3 p-3.5 rounded-lg bg-muted border border-border"
          >
            <component :is="item.icon" class="size-4 text-primary shrink-0" />
            <span class="text-sm text-foreground">{{ item.text }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- ── IaC / Architecture ─────────────────────────────────────── -->
    <section id="architecture" class="py-20 px-6 bg-muted/50 border-y border-border">
      <div class="max-w-5xl mx-auto">
        <div class="text-center mb-12">
          <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-2">Infrastructure as Code</p>
          <h2 class="text-2xl font-black tracking-tighter text-foreground">一行命令，节点即入网</h2>
          <p class="text-muted-foreground text-sm mt-2.5 max-w-md mx-auto">
            声明式指令实现节点自动发现与握手，复杂组网逻辑简化为一行部署命令。
          </p>
        </div>

        <div class="flex flex-col lg:flex-row gap-5">
          <!-- Steps -->
          <div class="lg:w-2/5 bg-card border border-border rounded-xl p-6 space-y-5">
            <div v-for="(step, i) in [
              { n: '01', title: '创建工作空间', desc: '定义网络逻辑边界，生成接入 Token。' },
              { n: '02', title: '运行 Join 命令', desc: '在目标机器一键执行，自动握手建隧。' },
              { n: '03', title: '拓扑自动同步', desc: '节点注册完成，控制面实时下发策略。' },
            ]" :key="i" class="flex items-start gap-3.5">
              <div class="size-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center text-[11px] font-black shrink-0 mt-0.5">
                {{ step.n }}
              </div>
              <div>
                <p class="text-sm font-semibold text-card-foreground">{{ step.title }}</p>
                <p class="text-xs text-muted-foreground mt-0.5 leading-relaxed">{{ step.desc }}</p>
              </div>
            </div>
          </div>

          <!-- Terminal -->
          <div class="lg:w-3/5 rounded-xl overflow-hidden border border-zinc-800">
            <div class="flex items-center gap-1.5 px-4 py-2.5 bg-zinc-900 border-b border-zinc-800">
              <div class="size-2.5 rounded-full bg-rose-500/70" />
              <div class="size-2.5 rounded-full bg-amber-400/70" />
              <div class="size-2.5 rounded-full bg-emerald-500/70" />
              <span class="ml-2 text-[11px] text-zinc-500 font-mono">bash</span>
            </div>
            <div class="bg-zinc-950 p-5 font-mono text-sm leading-7">
              <p><span class="text-zinc-600 select-none">#  </span><span class="text-zinc-500 italic">Standard Node Onboarding</span></p>
              <p><span class="text-zinc-500 select-none">$  </span><span class="text-emerald-400">curl -sSL https://get.wireflow.io \</span></p>
              <p><span class="text-zinc-700 select-none">   </span><span class="text-emerald-400">  | sudo bash -s -- join \</span></p>
              <p><span class="text-zinc-700 select-none">   </span><span class="text-emerald-400">  --token <span class="text-sky-400">wf_live_8s2k...92nz</span></span></p>
              <p class="mt-2"><span class="text-zinc-600 select-none">✓  </span><span class="text-emerald-500">Tunnel established · latency 12ms</span></p>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ── Pricing ───────────────────────────────────────────────── -->
    <section id="pricing" class="py-20 px-6">
      <div class="max-w-4xl mx-auto">
        <div class="text-center mb-12">
          <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-2">Pricing</p>
          <h2 class="text-2xl font-black tracking-tighter text-foreground">简单透明的定价</h2>
          <p class="text-muted-foreground text-sm mt-2.5 max-w-md mx-auto leading-relaxed">
            Community 版永久免费，Pro 版解锁企业级监控、SSO 与高级网络功能。
          </p>
        </div>

        <div class="grid md:grid-cols-2 gap-5">
          <!-- Community -->
          <div class="bg-card border border-border rounded-2xl p-8 flex flex-col">
            <div class="mb-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-3">Community</p>
              <div class="flex items-end gap-1.5 mb-2">
                <span class="text-4xl font-black tracking-tighter text-foreground">免费</span>
              </div>
              <p class="text-xs text-muted-foreground">永久免费 · 开源自托管</p>
            </div>

            <ul class="space-y-3 mb-8 flex-1">
              <li v-for="item in [
                '节点数无限制',
                'WireGuard 自动配置',
                'ACL 策略管理',
                'Network Peering',
                'K8s Operator / CRD',
                'REST API',
                'GitHub 社区支持',
              ]" :key="item" class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ item }}
              </li>
              <li v-for="item in [
                '监控仪表盘',
                'Dex OIDC / SSO',
                'TURN 中继服务器',
              ]" :key="item" class="flex items-center gap-2.5 text-sm text-muted-foreground/50 line-through">
                <X class="size-4 text-muted-foreground/30 shrink-0" />
                {{ item }}
              </li>
            </ul>

            <a href="https://github.com/francisxys" target="_blank" rel="noopener noreferrer">
              <Button variant="outline" class="w-full border-border" size="lg">
                查看源码
              </Button>
            </a>
          </div>

          <!-- Pro -->
          <div class="relative bg-card border-2 border-primary rounded-2xl p-8 flex flex-col shadow-lg shadow-primary/10">
            <!-- Badge -->
            <div class="absolute -top-3.5 left-1/2 -translate-x-1/2">
              <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-primary text-primary-foreground text-[11px] font-bold shadow-sm">
                <Crown class="size-3" /> 30 天免费试用
              </span>
            </div>

            <div class="mb-6">
              <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-3">Pro</p>
              <div class="flex items-end gap-1.5 mb-2">
                <span class="text-4xl font-black tracking-tighter text-foreground">$499</span>
                <span class="text-sm text-muted-foreground mb-1.5">/ 年起</span>
              </div>
              <p class="text-xs text-muted-foreground">25 节点起 · 私有部署 · License 授权</p>
            </div>

            <ul class="space-y-3 mb-8 flex-1">
              <li class="flex items-center gap-2.5 text-sm font-medium text-primary">
                <CheckCircle class="size-4 shrink-0" />
                包含所有 Community 功能
              </li>
              <li v-for="item in [
                '实时监控仪表盘',
                '网络拓扑可视化',
                '节点流量 / 延迟图表',
                '审计日志',
                'Dex OIDC / SSO 登录',
                'TURN 中继服务器',
                '遥测采集（VictoriaMetrics）',
                '邮件 / 工单技术支持',
              ]" :key="item" class="flex items-center gap-2.5 text-sm text-foreground">
                <CheckCircle class="size-4 text-emerald-500 shrink-0" />
                {{ item }}
              </li>
            </ul>

            <Button
              class="w-full gap-2 bg-primary hover:bg-primary/90 text-primary-foreground border-0 shadow-md shadow-primary/20"
              size="lg"
              @click="router.push('/auth/login')"
            >
              <Crown class="size-4" /> 申请试用
            </Button>
            <p class="text-center text-[11px] text-muted-foreground mt-3">无需信用卡 · 试用期结束后自动降级为 Community</p>
          </div>
        </div>

        <!-- Enterprise hint -->
        <div class="mt-5 flex items-center justify-center gap-2 text-sm text-muted-foreground">
          <span>节点数超过 500 或有特殊合规需求？</span>
          <a href="mailto:hello@wireflow.io" class="text-foreground font-medium hover:underline underline-offset-4 transition-colors">联系我们获取 Enterprise 报价 →</a>
        </div>
      </div>
    </section>

    <!-- ── CTA ────────────────────────────────────────────────────── -->
    <section id="quickstart" class="py-20 px-6">
      <div class="max-w-xl mx-auto text-center">
        <div class="size-14 rounded-2xl bg-primary/10 flex items-center justify-center mx-auto mb-5">
          <Network class="size-7 text-primary" />
        </div>
        <h2 class="text-2xl font-black tracking-tighter mb-3 text-foreground">准备好开始了吗？</h2>
        <p class="text-muted-foreground text-sm leading-relaxed mb-7 max-w-sm mx-auto">
          5 分钟内完成首个节点接入，无需修改防火墙规则，兼容任意云厂商与裸金属环境。
        </p>
        <div class="flex flex-col sm:flex-row gap-3 justify-center mb-8">
          <Button
            size="lg"
            class="gap-2 px-8 bg-primary hover:bg-primary/90 text-primary-foreground border-0 shadow-lg shadow-primary/20"
            @click="router.push('/manage/stepper')"
          >
            <Zap class="size-4" /> 立即开始接入
          </Button>
          <Button
            variant="outline"
            size="lg"
            class="gap-2 px-8 border-border"
            @click="router.push('/dashboard')"
          >
            查看控制台 <ArrowRight class="size-4" />
          </Button>
        </div>

        <div class="grid grid-cols-3 gap-2 text-left max-w-xs mx-auto">
          <div v-for="item in ['无需公网 IP', '端到端加密', '多平台支持', 'K8s 原生', '开箱即用', '社区开源']" :key="item"
            class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <CheckCircle class="size-3.5 text-emerald-500 shrink-0" />
            {{ item }}
          </div>
        </div>
      </div>
    </section>

    <!-- ── Footer ─────────────────────────────────────────────────── -->
    <footer class="border-t border-border px-6 py-7">
      <div class="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
        <div class="flex items-center gap-2">
          <img src="@/assets/logo.svg" class="size-5" alt="Wireflow" />
          <span class="text-sm font-black tracking-tighter text-muted-foreground">Wireflow</span>
        </div>
        <p class="text-[11px] text-muted-foreground font-mono uppercase tracking-widest">
          © 2026 Wireflow · 自动化网络编排平台 · WireGuard-Native
        </p>
        <div class="flex items-center gap-5 text-xs text-muted-foreground">
          <a href="#" class="hover:text-foreground transition-colors">文档</a>
          <a href="#pricing" class="hover:text-foreground transition-colors">定价</a>
          <a href="#" class="hover:text-foreground transition-colors">GitHub</a>
          <a href="#" class="hover:text-foreground transition-colors">社区</a>
        </div>
      </div>
    </footer>

  </div>
</template>
