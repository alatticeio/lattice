<script setup lang="ts">
import {onMounted} from 'vue'
import Icon from '../components/icons/Icon.vue'
import SideDrawer from "../components/SideDrawer.vue"
import Pagination from "../components/Pagination.vue"
import {useThemeStore} from '@/stores/useTheme'
import {useWorkspacePageStore} from "@/stores/workspacePage";

const workspacePageStore = useWorkspacePageStore()
const themeStore = useThemeStore()

onMounted(() => {
  workspacePageStore.actions.refresh()
})

</script>
<template>
  <div class="bg-base-200/50 transition-colors duration-300 text-base-content">
    <div class="max-w-[1440px] mx-auto p-4 lg:p-8 space-y-6">

      <header class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-6 border-b border-base-content/10">
        <div class="flex items-center gap-4">
          <div class="p-3 bg-primary rounded-2xl text-primary-content shadow-lg shadow-primary/20">
            <Icon name="workspace" :loading="workspacePageStore.loading" class="w-6 h-6"/>
          </div>
          <div>
            <h1 class="text-2xl font-black tracking-tighter uppercase">工作空间</h1>
            <p class="text-xs font-bold opacity-40 uppercase tracking-widest">
              资源池总数：<span class="text-primary">{{ workspacePageStore.total || 0 }}</span>
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <button class="btn btn-ghost bg-base-100 border-base-content/5 rounded-xl hover:bg-base-200" @click="workspacePageStore.actions.refresh">
            <Icon name="refresh" :loading="workspacePageStore.loading" :class="['w-4 h-4']" />
          </button>
          <button @click="workspacePageStore.actions.openDrawer('create')" class="btn btn-primary rounded-xl px-6 shadow-lg shadow-primary/20">
            <Icon name="plus" class="w-4 h-4 mr-2"/>
            新建空间
          </button>
        </div>
      </header>

      <main>
        <div v-if="workspacePageStore.loading && workspacePageStore.rows.length === 0" class="grid grid-cols-1 md:grid-cols-3 xl:grid-cols-4 gap-6">
          <div v-for="i in 4" :key="i" class="h-64 bg-base-300/50 rounded-3xl animate-pulse"></div>
        </div>

        <div v-if="workspacePageStore.rows.length > 0" class="grid grid-cols-1 md:grid-cols-3 xl:grid-cols-4 gap-6">
          <div v-for="ws in workspacePageStore.rows" :key="ws.id"
               class="group bg-base-100 rounded-3xl border border-base-content/5 shadow-sm hover:shadow-xl hover:border-primary/30 transition-all duration-500 flex flex-col relative">

            <div class="p-5 flex justify-between items-start">
              <div class="flex items-center gap-4 min-w-0">
                <div :class="['w-10 h-10 flex items-center justify-center rounded-xl text-xs font-black text-white shadow-inner transition-transform group-hover:scale-110', themeStore.actions.getAvatarColor(ws.displayName)]">
                  {{ themeStore.actions.getFirstChar(ws.displayName) }}
                </div>
                <div class="min-w-0">
                  <h3 class="font-black text-sm truncate group-hover:text-primary transition-colors uppercase">{{ ws.displayName }}</h3>
                  <span class="text-[10px] font-mono opacity-40 block truncate tracking-tighter">标识符: {{ ws.id.substring(0,8) }}</span>
                </div>
              </div>
              <div class="badge badge-xs" :class="ws.status === 'healthy' ? 'badge-success animate-pulse' : 'badge-warning'"></div>
            </div>

            <div class="px-5 py-3 flex items-center justify-between bg-base-200/30 border-y border-base-content/5">
              <div class="text-center flex-1">
                <div class="text-[9px] uppercase font-black opacity-30 tracking-widest">节点数</div>
                <div class="text-sm font-black">{{ ws.nodeCount }}</div>
              </div>
              <div class="w-px h-6 bg-base-content/5"></div>
              <div class="text-center flex-1">
                <div class="text-[9px] uppercase font-black opacity-30 tracking-widest">流量状态</div>
                <div class="text-sm font-black text-primary">活跃</div>
              </div>
            </div>

            <div class="px-5 pt-4">
              <div class="flex justify-between text-[9px] font-black uppercase mb-1.5 opacity-40">
                <span>配额使用率</span>
                <span>{{ ws.nodeCount }} / {{ ws.maxNodes }}</span>
              </div>
              <div class="w-full bg-base-300 h-1.5 rounded-full overflow-hidden">
                <div class="h-full bg-primary transition-all duration-1000"
                     :style="{ width: `${(ws.nodeCount / ws.maxNodes) * 100}%` }"></div>
              </div>
            </div>

              <div class="p-4 mt-2 flex justify-between items-center rounded-b-3xl bg-base-200/50 group-hover:bg-transparent transition-colors">
                <div class="flex gap-1">
                  <div class="tooltip tooltip-top" data-tip="编辑">
                    <button
                        @click="workspacePageStore.actions.openDrawer('edit')"
                        class="btn btn-ghost btn-xs btn-square text-base-content/50 hover:text-primary hover:bg-primary/10"
                    >
                      <Icon name="edit" class="w-4 h-4"/>
                    </button>
                  </div>

                  <div class="tooltip tooltip-top" data-tip="销毁">
                    <button
                        @click="workspacePageStore.actions.handleDelete(ws)"
                        class="btn btn-ghost btn-xs text-error/90 hover:text-error hover:bg-error/40"
                    >
                      <Icon name="trash" class="w-4 h-4"/>
                    </button>
                  </div>
                </div>

                <button @click="workspacePageStore.actions.enterWorkspace(ws)" class="btn btn-primary btn-xs rounded-lg px-4 font-black shadow-md">
                  管理节点
                  <Icon name="arrow-right" class="w-3 h-3 ml-1"/>
                </button>
              </div>


            </div>
          </div>
<!--        </div>-->

        <div v-else-if="!workspacePageStore.loading" class="py-32 text-center">
          <div class="inline-flex p-8 bg-base-300/30 rounded-full mb-6">
            <Icon name="workspace" class="w-16 h-16 opacity-10"/>
          </div>
          <p class="text-sm font-black opacity-30 uppercase tracking-[0.2em]">未发现活跃的工作空间</p>
          <button @click="workspacePageStore.actions.openDrawer('create')" class="btn btn-link no-underline text-primary mt-2">立即创建</button>
        </div>
      </main>

      <footer class="flex flex-col sm:flex-row justify-between items-center pt-8 border-t border-base-content/5 gap-4">
        <div class="text-[10px] font-black uppercase opacity-30 tracking-tighter">
          共 {{ workspacePageStore.total }} 个实体，当前显示 {{ workspacePageStore.rows.length }} 个
        </div>
        <Pagination v-model:page="workspacePageStore.params.page" v-model:pageSize="workspacePageStore.params.pageSize" :total="workspacePageStore.total" />
      </footer>

      <SideDrawer
          v-model="workspacePageStore.ui.isDrawerOpen"
          :title="workspacePageStore.ui.drawerType === 'create' ? '初始化新空间' : '空间详情'"
          class="overflow-hidden"
      >
        <div class="flex flex-col h-[85dvh] bg-base-100 overflow-hidden border-none shadow-none">

          <div class="flex-1 overflow-y-auto custom-scrollbar min-h-0 h-0">
            <div class="p-6 space-y-6">
              <div class="space-y-2">
                <label class="text-[10px] font-black opacity-40 uppercase tracking-widest px-1">显示名称</label>
                <input v-model="workspacePageStore.form.displayName" type="text" placeholder="例如：北京-边缘算力池"
                       class="input input-bordered w-full bg-base-200 border-none focus:ring-2 focus:ring-primary/20 font-bold" />
              </div>

              <div class="space-y-2">
                <label class="text-[10px] font-black opacity-40 uppercase tracking-widest px-1">命名空间别名 (K8s Slug)</label>
                <input v-model="workspacePageStore.form.slug" type="text" placeholder="wf-namespace-01"
                       class="input input-bordered w-full bg-base-200 border-none font-mono text-xs focus:ring-2 focus:ring-primary/20" />
              </div>

              <div class="space-y-2">
                <label class="text-[10px] font-black opacity-40 uppercase tracking-widest px-1">资源配额</label>
                <select v-model="workspacePageStore.form.maxNodeCount" class="select select-bordered w-full bg-base-200 border-none font-bold">
                  <option :value="20">标准版 (20 节点)</option>
                  <option :value="50">高级版 (50 节点)</option>
                  <option :value="100">企业版 (100 节点)</option>
                </select>
              </div>

              <div class="p-4 bg-primary/5 rounded-2xl border border-primary/10">
                <div class="flex gap-3">
                  <Icon name="info" class="w-4 h-4 text-primary shrink-0 mt-0.5"/>
                  <p class="text-[10px] leading-relaxed font-bold opacity-60 italic">
                    新空间将自动继承全局 'default-deny' 出口策略，以增强零信任安全保障。
                  </p>
                </div>
              </div>

              <div class="h-2"></div>
            </div>
          </div>

          <div class="shrink-0 p-6 bg-base-100 border-t border-base-content/5">
            <button
                @click="workspacePageStore.actions.handleCreate"
                :disabled="!workspacePageStore.form.displayName || workspacePageStore.ui.isSubmitting"
                class="group relative btn btn-primary w-full h-14 rounded-2xl border-none font-black uppercase tracking-[0.2em] transition-all duration-300
           bg-gradient-to-r from-primary to-primary-focus
           hover:shadow-[0_10px_30px_-10px_rgba(var(--p),0.5)]
           active:scale-[0.98] disabled:bg-base-300 disabled:opacity-50"
            >
              <div class="absolute inset-0 rounded-2xl bg-white/10 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none"></div>

              <div class="flex items-center justify-center gap-3">
                <template v-if="!workspacePageStore.ui.isSubmitting">
                  <span class="text-sm">配置并发布空间</span>
                  <Icon name="arrow-right" class="w-4 h-4 transition-transform group-hover:translate-x-1"/>
                </template>

                <template v-else>
                  <span class="loading loading-spinner loading-sm"></span>
                  <span class="text-xs lowercase opacity-70 tracking-normal">deploying to cluster...</span>
                </template>
              </div>
            </button>
          </div>
        </div>
      </SideDrawer>
    </div>
  </div>
</template>

<style scoped>
/* 针对不同浏览器的隐藏滚动条处理 */
.custom-scrollbar {
  -ms-overflow-style: none; /* IE and Edge */
  scrollbar-width: none;    /* Firefox */
}
.custom-scrollbar::-webkit-scrollbar {
  display: none;            /* Chrome, Safari, Opera */
}
</style>