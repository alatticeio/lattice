<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import Icon from './icons/Icon.vue' // 确保你的图标组件路径正确

const route = useRoute()
const wsId = computed(() => route.params.wsId as string || 'Select Workspace')
</script>

<template>
  <nav class="sticky top-0 z-[100] w-full border-b border-slate-200/60 dark:border-white/5 bg-base-100/80 dark:bg-slate-900/80 backdrop-blur-md">
    <div class="max-w-[1440px] mx-auto px-4 h-14 flex items-center justify-between">

      <div class="flex items-center gap-6">
        <router-link to="/" class="flex items-center gap-2 group">
          <div class="w-7 h-7 bg-blue-600 rounded-lg flex items-center justify-center shadow-lg shadow-blue-500/30 group-hover:scale-105 transition-transform">
            <Icon name="zap" class="w-4 h-4 text-white" />
          </div>
          <span class="text-sm font-black tracking-tighter text-base-content dark:text-white uppercase">
            WireFlow
          </span>
        </router-link>

        <div class="h-4 w-px bg-slate-200 dark:bg-base-100/10 hidden md:block"></div>

        <div class="dropdown dropdown-bottom">
          <div tabindex="0" role="button" class="flex items-center gap-2 px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-base-100/5 rounded-xl transition-all group">
            <div class="w-2 h-2 rounded-full bg-emerald-500"></div>
            <span class="text-[11px] font-bold text-base-content/70 dark:text-slate-300 tracking-tight">
              {{ wsId }}
            </span>
            <Icon name="chevron-down" class="w-3 h-3 text-base-content/40 group-hover:text-base-content/70 transition-colors" />
          </div>
          <ul tabindex="0" class="dropdown-content menu p-2 shadow-2xl bg-base-100 dark:bg-slate-800 rounded-2xl w-60 border border-slate-200 dark:border-white/5 mt-2">
            <li class="menu-title px-3 py-2 text-[10px] font-black uppercase tracking-widest text-base-content/40">切换工作空间</li>
            <li><a class="rounded-lg text-xs font-bold py-2.5">上海生产环境 (Default)</a></li>
            <li><a class="rounded-lg text-xs font-bold py-2.5 text-base-content/60">北京研发中心</a></li>
            <div class="divider my-1 opacity-50"></div>
            <li>
              <a class="rounded-lg text-xs font-bold py-2.5 text-blue-600">
                <Icon name="plus" class="w-3.5 h-3.5" /> 创建新空间
              </a>
            </li>
          </ul>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div class="hidden lg:flex items-center gap-2 px-3 py-1.5 bg-slate-100 dark:bg-base-100/5 border border-transparent hover:border-slate-200 dark:hover:border-white/10 rounded-xl transition-all cursor-pointer group w-48">
          <Icon name="search" class="w-3.5 h-3.5 text-base-content/40" />
          <span class="text-[11px] font-bold text-base-content/40 flex-1">Quick Search...</span>
          <kbd class="kbd kbd-xs bg-base-100 dark:bg-slate-900 border-slate-200 dark:border-white/10 text-[9px] font-black opacity-50">⌘ K</kbd>
        </div>

        <div class="flex items-center gap-1 border-l border-slate-200 dark:border-white/10 ml-2 pl-3">
          <button class="btn btn-ghost btn-sm btn-square rounded-xl text-base-content/60 relative">
            <Icon name="bell" class="w-4 h-4" />
            <span class="absolute top-2 right-2 w-1.5 h-1.5 bg-error rounded-full border-2 border-white dark:border-slate-900"></span>
          </button>

          <div class="dropdown dropdown-end ml-1">
            <div tabindex="0" role="button" class="flex items-center gap-2 hover:bg-slate-100 dark:hover:bg-base-100/5 p-1 rounded-xl transition-all">
              <div class="avatar">
                <div class="w-8 rounded-lg shadow-sm">
                  <img src="https://api.dicebear.com/7.x/avataaars/svg?seed=Felix" />
                </div>
              </div>
            </div>
            <ul tabindex="0" class="dropdown-content menu p-2 shadow-2xl bg-base-100 dark:bg-slate-800 rounded-2xl w-52 border border-slate-200 dark:border-white/5 mt-3">
              <div class="px-4 py-3 border-b border-slate-50 dark:border-white/5 mb-2">
                <p class="text-[10px] font-black text-base-content/40 uppercase tracking-widest">Signed in as</p>
                <p class="text-xs font-bold text-slate-700 dark:text-slate-200 truncate">admin@wireflow.io</p>
              </div>
              <li><a class="rounded-lg text-[11px] font-bold py-2.5 hover:bg-slate-50">个人中心</a></li>
              <li><a class="rounded-lg text-[11px] font-bold py-2.5 hover:bg-slate-50">API 令牌</a></li>
              <li><a class="rounded-lg text-[11px] font-bold py-2.5 hover:bg-slate-50 text-error">退出登录</a></li>
            </ul>
          </div>
        </div>
      </div>

    </div>
  </nav>
</template>

<style scoped>
/* 针对 MacBook 14' 优化的极细边框 */
.divider {
  @apply h-px bg-slate-100 dark:bg-base-100/5;
}
</style>