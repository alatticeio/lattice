<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()

const crumbs = computed(() => {
  const matched = route.matched
      .filter(r => r.meta && r.meta.title)
      .map(r => ({
        title: r.meta.title as string,
        path: r.path,
      }))

  if (matched.length > 0) return matched

  const pathParts = route.path.split('/').filter(Boolean)
  const fallbackTitle = pathParts.length > 0
      ? pathParts[pathParts.length - 1].charAt(0).toUpperCase() + pathParts[pathParts.length - 1].slice(1)
      : 'Dashboard'

  return [{ title: fallbackTitle, path: route.path }]
})

const pageTitle = computed(() => {
  const last = crumbs.value[crumbs.value.length - 1]
  return last?.title || ''
})
</script>

<template>
  <div class="flex items-center justify-between w-full">
    <nav class="flex items-center gap-1 p-1.5 bg-base-100/50 dark:bg-slate-900/50 backdrop-blur-xl rounded-2xl border border-slate-200/60 dark:border-white/5 shadow-sm">

      <router-link
          to="/dashboard"
          class="btn btn-ghost btn-sm btn-square rounded-xl hover:bg-base-100 dark:hover:bg-base-100/5 text-base-content/40 dark:text-base-content/60 hover:text-blue-600 dark:hover:text-blue-400 transition-all"
          title="Dashboard"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"></path>
          <polyline points="9 22 9 12 15 12 15 22"></polyline>
        </svg>
      </router-link>

      <div class="flex items-center gap-1 pr-3">
        <ol class="flex items-center">
          <li v-for="(c, i) in crumbs" :key="c.path + ':' + i" class="flex items-center">
            <div v-if="i !== 0" class="mx-1.5 text-slate-300 dark:text-slate-700">
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3.5">
                <path d="M9 18l6-6-6-6"></path>
              </svg>
            </div>

            <router-link
                v-if="i !== crumbs.length - 1"
                :to="c.path"
                class="px-2 py-1 text-[11px] font-bold text-base-content/60 dark:text-base-content/40 hover:text-blue-600 dark:hover:text-blue-400 transition-colors rounded-lg"
            >
              {{ c.title }}
            </router-link>

            <span v-else class="px-2 py-1 text-[11px] font-black text-base-content dark:text-slate-100 tracking-tight">
              {{ c.title }}
            </span>
          </li>
        </ol>
      </div>
    </nav>

    <div class="hidden md:flex items-center gap-4">
      <div class="flex flex-col items-end gap-0.5">
        <span class="text-[9px] font-black uppercase tracking-[0.2em] text-base-content/40 dark:text-base-content/60 leading-none">
          Live Session
        </span>
        <span class="text-[10px] font-bold text-slate-800 dark:text-slate-200 leading-none">
          {{ pageTitle }}
        </span>
      </div>
      <div class="relative flex h-2 w-2">
        <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
        <span class="relative inline-flex rounded-full h-2 w-2 bg-primary"></span>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 确保在切换主题时有平滑过渡 */
nav {
  transition: background-color 0.3s, border-color 0.3s;
}
</style>