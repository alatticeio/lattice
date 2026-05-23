<script setup lang="ts">
import { Network, ShieldCheck, Search, Zap, Terminal, GitBranch } from 'lucide-vue-next'

const emit = defineEmits<{ select: [prompt: string] }>()

const groups = [
  {
    label: '常见问题',
    items: [
      { icon: Search,  text: '现在哪些 Peer 离线了？' },
      { icon: ShieldCheck, text: '分析当前工作区的安全策略' },
    ],
  },
  {
    label: '网络管理',
    items: [
      { icon: Network, text: '列出当前所有网络和它们的 CIDR' },
      { icon: Zap,     text: '为什么两个 Peer 之间无法通信？' },
    ],
  },
  {
    label: '运维诊断',
    items: [
      { icon: Terminal,   text: '查看最近的连接失败事件' },
      { icon: GitBranch,  text: '当前有哪些活跃的中继节点？' },
    ],
  },
]
</script>

<template>
  <div class="flex h-full flex-col items-center justify-center px-6">
    <!-- Heading -->
    <div class="mb-8 text-center">
      <div class="mx-auto mb-4 flex size-12 items-center justify-center rounded-2xl bg-primary/10 ring-1 ring-primary/20">
        <svg class="size-6 text-primary" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
        </svg>
      </div>
      <h2 class="text-2xl font-semibold tracking-tight">Lattice AI</h2>
      <p class="mt-2 text-sm text-muted-foreground">用自然语言管理 WireGuard 网络</p>
    </div>

    <!-- Grouped prompts -->
    <div class="w-full max-w-lg space-y-5">
      <div v-for="group in groups" :key="group.label">
        <p class="mb-2 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">
          {{ group.label }}
        </p>
        <div class="flex flex-col gap-1.5">
          <button
            v-for="item in group.items"
            :key="item.text"
            class="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-2.5 text-left text-sm transition-all hover:border-primary/40 hover:bg-primary/5 hover:shadow-sm"
            @click="emit('select', item.text)"
          >
            <component :is="item.icon" class="size-3.5 shrink-0 text-muted-foreground" />
            <span class="text-foreground/80">{{ item.text }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
