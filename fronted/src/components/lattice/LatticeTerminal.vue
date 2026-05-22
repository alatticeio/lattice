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
        <span :class="`lattice-terminal__${line.cls ?? 'cmd'}`">{{ line.text }}</span>
      </p>
    </div>
  </div>
</template>
