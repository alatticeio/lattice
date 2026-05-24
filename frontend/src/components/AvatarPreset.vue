<script setup lang="ts">
import { computed } from 'vue'
import { getPreset, isPreset } from '@/lib/avatarPresets'

const props = defineProps<{
  avatarUrl: string
  size?: number
  fallback?: string
}>()

const preset = computed(() => isPreset(props.avatarUrl) ? getPreset(props.avatarUrl) : null)
const sizePx = computed(() => `${props.size ?? 32}px`)
const fallbackFontSize = computed(() => `${Math.round((props.size ?? 32) * 0.38)}px`)
</script>

<template>
  <img
    v-if="preset"
    :src="preset.url"
    :alt="preset.label"
    :style="{ width: sizePx, height: sizePx }"
    class="rounded-lg shrink-0 object-cover"
  />
  <div
    v-else
    :style="{ width: sizePx, height: sizePx, fontSize: fallbackFontSize }"
    class="rounded-lg bg-primary flex items-center justify-center shrink-0 text-primary-foreground font-semibold select-none"
  >
    {{ (fallback ?? '?').charAt(0).toUpperCase() }}
  </div>
</template>
