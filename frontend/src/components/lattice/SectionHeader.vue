<script setup lang="ts">
defineProps<{
  tag: string
  title: string
  subtitle?: string
  subtitleClass?: string
}>()
</script>

<template>
  <div class="text-center mb-12">
    <p class="text-xs font-black uppercase tracking-widest text-muted-foreground mb-2">{{ tag }}</p>
    <h2 class="text-2xl md:text-3xl font-black tracking-tighter text-foreground">{{ title }}</h2>
    <p v-if="subtitle" :class="['text-muted-foreground text-sm mt-2.5 mx-auto leading-relaxed', subtitleClass ?? 'max-w-lg']">
      {{ subtitle }}
    </p>
  </div>
</template>
