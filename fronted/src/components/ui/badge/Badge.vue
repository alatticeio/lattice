<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "@/lib/utils"

const props = defineProps<{
  class?: HTMLAttributes["class"]
  variant?: "default" | "secondary" | "destructive" | "outline"
}>()
</script>

<template>
  <span
    :class="cn(
      'inline-flex items-center rounded-md px-2.5 py-0.5 text-xs font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
      variant === 'default' && 'border-transparent bg-primary text-primary-foreground',
      variant === 'secondary' && 'border-transparent bg-secondary text-secondary-foreground',
      variant === 'destructive' && 'border-transparent bg-destructive text-destructive-foreground',
      variant === 'outline' && 'text-foreground border border-border',
      props.class,
    )"
  >
    <slot />
  </span>
</template>
