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
