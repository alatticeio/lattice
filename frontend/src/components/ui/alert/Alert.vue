<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "@/lib/utils"

const props = defineProps<{
  class?: HTMLAttributes["class"]
  variant?: "default" | "destructive"
}>()
</script>

<template>
  <div
    role="alert"
    :class="cn(
      'relative w-full rounded-lg border px-4 py-3 text-sm grid has-[>svg]:grid-cols-[calc(var(--spacing)*4)_1fr] grid-cols-[0_1fr] gap-x-3 gap-y-0.5 items-start [&>svg]:size-4 [&>svg]:translate-y-0.5 [&>svg]:self-start',
      variant === 'destructive' && 'text-destructive dark:border-destructive/50 [&>svg]:text-destructive',
      variant === 'default' && 'bg-background text-foreground [&>svg]:text-foreground',
      props.class,
    )"
  >
    <slot />
  </div>
</template>
