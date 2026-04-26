<script setup lang="ts">
import { Network, ShieldCheck, Search, Zap } from 'lucide-vue-next'

const emit = defineEmits<{ select: [prompt: string] }>()

const prompts = [
  { icon: Search,       label: '查询离线',   text: '现在哪些 Peer 离线了？' },
  { icon: Network,      label: '网络概览',   text: '列出当前所有网络和它们的 CIDR' },
  { icon: ShieldCheck,  label: '安全审计',   text: '分析当前工作区的安全策略' },
  { icon: Zap,          label: '连通性诊断', text: '为什么两个 Peer 之间无法通信？' },
]
</script>

<template>
  <div class="flex h-full flex-col items-center justify-center px-6">
    <!-- Heading -->
    <div class="mb-10 text-center">
      <div class="mx-auto mb-4 flex size-12 items-center justify-center rounded-2xl bg-primary/10 ring-1 ring-primary/20">
        <svg class="size-6 text-primary" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
        </svg>
      </div>
      <h2 class="text-2xl font-semibold tracking-tight">Wireflow AI</h2>
      <p class="mt-2 text-sm text-muted-foreground">你的网络运维助手，用自然语言管理 WireGuard 网络</p>
    </div>

    <!-- Prompt grid -->
    <div class="grid w-full max-w-xl grid-cols-2 gap-3">
      <button
        v-for="p in prompts"
        :key="p.text"
        class="group flex flex-col items-start gap-2.5 rounded-xl border border-border bg-card p-4 text-left transition-all hover:border-primary/40 hover:bg-primary/5 hover:shadow-sm"
        @click="emit('select', p.text)"
      >
        <component
          :is="p.icon"
          class="size-4 text-muted-foreground transition-colors group-hover:text-primary"
        />
        <div>
          <p class="text-xs font-medium text-foreground">{{ p.label }}</p>
          <p class="mt-0.5 text-xs text-muted-foreground leading-snug">{{ p.text }}</p>
        </div>
      </button>
    </div>
  </div>
</template>
