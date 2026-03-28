<script setup lang="ts">
import {useNavbarStore} from "@/stores/pages/topNavbar";
import {useUserStore} from "@/stores/user";
import {useTheme} from "@/composables/useTheme";

const userStore = useUserStore()
const {theme, toggleTheme} = useTheme()
const navStore = useNavbarStore()

</script>
<template>
  <header class="w-full sticky top-0 z-[50] transition-all duration-300
                 bg-base-100/80 backdrop-blur-xl
                 border-b border-base-content/10">

    <div class="mx-auto max-w-[1440px] px-4 lg:px-8">
      <div class="navbar px-0 h-14 min-h-[3.5rem]">

        <div class="navbar-start gap-3">
          <router-link to="/dashboard" class="flex items-center gap-2 group">
            <div class="w-8 h-8 bg-primary rounded-lg flex items-center justify-center shadow-lg shadow-primary/20 group-hover:scale-105 transition-transform duration-500">
              <svg width="18" height="18" viewBox="0 0 32 32" fill="none" class="text-primary-content">
                <path d="M18 6L10 18H16L14 26L22 14H16L18 6Z" fill="currentColor"/>
              </svg>
            </div>
            <span class="text-base font-black tracking-tighter uppercase text-base-content hidden md:block">Wireflow</span>
          </router-link>

          <div class="h-4 w-px bg-base-content/10 mx-1 hidden sm:block"></div>

          <div class="dropdown">
            <div tabindex="0" role="button"
                 class="flex items-center gap-2.5 px-3 py-1.5 rounded-xl hover:bg-base-content/5 transition-all cursor-pointer group border border-transparent hover:border-base-content/10">
              <div class="w-6 h-6 rounded-md bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center text-[10px] font-black text-white shadow-sm shadow-indigo-500/20 uppercase transition-transform group-hover:rotate-3">
                {{ navStore.currentWorkspace?.displayName.substring(0, 1) }}
              </div>
              <div class="flex flex-col items-start leading-none">
                <span class="text-[11px] font-black text-base-content/30 uppercase tracking-[0.1em] scale-90 origin-left mb-0.5">Workspace</span>
                <span class="text-[12px] font-bold text-base-content/80 max-w-[100px] truncate">
            {{ navStore.currentWorkspace?.displayName }}
          </span>
              </div>
              <svg class="w-3.5 h-3.5 opacity-20 group-hover:opacity-100 transition-all group-hover:translate-y-0.5" viewBox="0 0 24 24" fill="none"
                   stroke="currentColor" stroke-width="3">
                <path d="M6 9l6 6 6-6"></path>
              </svg>
            </div>

            <ul tabindex="0"
                class="dropdown-content menu bg-base-100/95 backdrop-blur-2xl rounded-[1.5rem] z-[100] mt-3 w-64 p-2 shadow-[0_20px_50px_rgba(0,0,0,0.1)] border border-base-content/5 animate-in fade-in zoom-in duration-200">
              <li class="menu-title px-4 py-3 text-[10px] font-black uppercase tracking-[0.2em] opacity-40">
                您的工作空间
              </li>

              <div class="space-y-1 px-1">
                <li v-for="ws in navStore.rows" :key="ws.id">
                  <a @click="navStore.actions.switchWorkspace(ws)"
                     :class="{ 'bg-primary/10 text-primary shadow-sm': navStore.currentWsId === ws.id }"
                     class="flex items-center gap-3 p-2.5 rounded-xl hover:bg-base-content/5 transition-all group/item">
                    <div class="relative">
                      <div class="w-9 h-9 rounded-lg bg-base-200 flex items-center justify-center font-bold text-sm transition-colors group-hover/item:bg-base-300">
                        {{ ws.displayName[0] }}
                      </div>
                      <div v-if="navStore.currentWsId === ws.id" class="absolute -top-1 -right-1 w-2.5 h-2.5 bg-primary rounded-full border-2 border-base-100"></div>
                    </div>
                    <div class="flex flex-col">
                      <span class="text-sm font-bold">{{ ws.displayName }}</span>
                      <span class="text-[10px] font-medium opacity-40">Active Nodes: 12</span>
                    </div>
                    <Icon v-if="navStore.currentWsId === ws.id" name="check" class="ml-auto w-4 h-4 text-primary" />
                  </a>
                </li>
              </div>

              <div class="divider my-2 opacity-5"></div>

              <li class="px-1">
                <router-link to="/workspace/settings"
                             class="flex items-center justify-center gap-2 py-3 bg-base-200/50 rounded-xl text-[11px] font-black uppercase tracking-widest hover:bg-primary hover:text-primary-content transition-all shadow-inner">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path d="M12 6v6m0 0v6m0-6h6m-6 0H6" stroke-width="3"/></svg>
                  管理所有空间
                </router-link>
              </li>
            </ul>
          </div>
        </div>

        <div class="navbar-center hidden lg:flex">
          <div class="relative group">
            <div class="absolute inset-y-0 left-3.5 flex items-center pointer-events-none opacity-40">
              <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                <circle cx="11" cy="11" r="7"></circle>
                <path d="M21 21l-4.3-4.3"></path>
              </svg>
            </div>
            <input
                type="text"
                placeholder="搜索节点、策略或文档..."
                class="w-[300px] h-9 pl-9 pr-12 bg-base-200/50 border-none rounded-xl text-[11px] transition-all
                       focus:ring-2 focus:ring-primary/20 focus:w-[360px] outline-none font-medium text-base-content/70"
            />
            <div class="absolute inset-y-0 right-3 flex items-center gap-1 opacity-30 group-focus-within:opacity-100 transition-opacity">
              <kbd class="kbd kbd-xs bg-base-100 border-none font-sans text-[9px] px-1.5 font-bold">⌘</kbd>
              <kbd class="kbd kbd-xs bg-base-100 border-none font-sans text-[9px] px-1.5 font-bold">K</kbd>
            </div>
          </div>
        </div>

        <div class="navbar-end gap-1.5">
          <button @click="toggleTheme"
                  class="btn btn-ghost btn-sm btn-square rounded-xl text-base-content/60 hover:text-primary transition-colors">
            <svg v-if="theme === 'light'" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M12 2v2m0 16v2m-7.07-17.07l1.41 1.41m11.32 11.32l1.41 1.41M2 12h2m16 0h2m-17.07 7.07l1.41-1.41m11.32-11.32l1.41-1.41"></path>
            </svg>
            <svg v-else class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M21 12.8A9 9 0 1111.2 3a7 7 0 109.8 9.8z"></path>
            </svg>
          </button>

          <div class="indicator">
            <span class="indicator-item w-1.5 h-1.5 bg-primary rounded-full top-3 right-3 border border-base-100"></span>
            <button class="btn btn-ghost btn-sm btn-square rounded-xl text-base-content/60">
              <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="M18 8a6 6 0 10-12 0c0 7-3 7-3 7h18s-3 0-3-7M13.73 21a2 2 0 01-3.46 0"></path>
              </svg>
            </button>
          </div>

          <div class="dropdown dropdown-end ml-1">
            <div tabindex="0" role="button"
                 class="btn btn-ghost h-9 px-1.5 rounded-xl hover:bg-base-content/5 flex items-center gap-2 transition-all">
              <div class="avatar">
                <div class="w-7 h-7 rounded-lg ring-1 ring-base-content/10 overflow-hidden shadow-sm">
                  <img :src="`https://ui-avatars.com/api/?name=${userStore.userInfo?.username || 'User'}&background=random`"/>
                </div>
              </div>
              <svg class="w-3 h-3 opacity-20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                <path d="M6 9l6 6 6-6"></path>
              </svg>
            </div>

            <ul tabindex="0"
                class="dropdown-content menu bg-base-100 rounded-2xl z-50 mt-3 w-60 p-2 shadow-2xl border border-base-content/5 animate-fade-in">
              <div class="px-4 py-3 mb-2 bg-base-200/50 rounded-xl">
                <div class="flex items-center justify-between mb-1">
                  <p class="text-[9px] font-black text-base-content/40 uppercase tracking-widest">当前登录</p>
                  <span class="px-1.5 py-0.5 bg-primary text-primary-content text-[8px] font-black rounded uppercase">
                    {{ userStore.userInfo?.role }}
                  </span>
                </div>
                <p class="text-xs font-bold text-base-content truncate">{{ userStore.userInfo?.username }}</p>
                <p class="text-[10px] text-base-content/40 truncate mt-0.5">{{ userStore.userInfo?.email }}</p>
              </div>

              <li><router-link to="/profile" class="rounded-lg py-2 text-[11px] font-bold hover:bg-base-content/5">个人中心</router-link></li>
              <li><router-link to="/settings" class="rounded-lg py-2 text-[11px] font-bold hover:bg-base-content/5">账户设置</router-link></li>

              <div class="divider my-1 opacity-5"></div>

              <li>
                <button @click="navStore.actions.handleLogout"
                        class="text-error hover:bg-error/10 rounded-lg py-2 text-[11px] font-black uppercase tracking-widest">
                  安全退出
                </button>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>