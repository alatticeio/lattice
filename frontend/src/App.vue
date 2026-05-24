<script setup lang="ts">
import Loading from '@/components/loading.vue'
import 'vue-sonner/style.css'
import { Toaster } from '@/components/ui/sonner'
import { useSystemTheme } from '@/composables/use-system-theme'

useSystemTheme()
</script>

<template>
  <Toaster position="top-center"/>

  <Suspense>
    <router-view v-slot="{ Component, route }">
      <component :is="Component" :key="route" />
    </router-view>

    <template #fallback>
      <Loading />
    </template>
  </Suspense>
</template>
