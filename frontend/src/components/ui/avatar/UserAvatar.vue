<script setup lang="ts">
import { computed } from 'vue'
import { Avatar, AvatarImage, AvatarFallback } from '@/components/ui/avatar'

const props = defineProps<{
  name?: string
  src?: string
  class?: string
}>()

// Deterministic color palette — enough variety, stays readable on white text
const COLORS = [
  '#3b82f6', // blue-500
  '#8b5cf6', // violet-500
  '#10b981', // emerald-500
  '#f97316', // orange-500
  '#f43f5e', // rose-500
  '#06b6d4', // cyan-500
  '#6366f1', // indigo-500
  '#ec4899', // pink-500
  '#84cc16', // lime-500
  '#14b8a6', // teal-500
]

const bgColor = computed(() => {
  const s = props.name ?? ''
  if (!s) return COLORS[0]
  let hash = 0
  for (let i = 0; i < s.length; i++) hash = (hash * 31 + s.charCodeAt(i)) & 0xffff
  return COLORS[hash % COLORS.length]
})

const initials = computed(() => {
  const s = (props.name ?? '').trim()
  if (!s) return '?'
  const parts = s.split(/[\s._\-@]+/).filter(Boolean)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
  return s.slice(0, 2).toUpperCase()
})
</script>

<template>
  <Avatar :class="props.class">
    <AvatarImage v-if="src" :src="src" :alt="name ?? ''" />
    <AvatarFallback
      class="text-xs font-bold text-white rounded-full"
      :style="{ backgroundColor: bgColor }"
    >
      {{ initials }}
    </AvatarFallback>
  </Avatar>
</template>
