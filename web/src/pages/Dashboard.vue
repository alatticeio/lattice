<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useUserDashboardStore } from '@/stores/useDashboard'

const dashboardStore = useUserDashboardStore()

// 语义化颜色映射：将后端下发的 tone 转换为具体的 CSS 类
const toneStyles: Record<string, string> = {
  emerald: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
  amber: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  blue: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
}

// 自动刷新逻辑
onMounted(() => {
  dashboardStore.refresh()
  // 可选：开启定时轮询
  // setInterval(() => dashboardStore.refresh(), 5000)
})

</script>

<template>
  <div class="max-w-[1600px] mx-auto p-4 lg:p-6 space-y-6 animate-fade-in text-base-content">

    <div class="flex flex-col md:flex-row justify-between items-end gap-4 border-b border-base-content/10 pb-6">
      <div class="space-y-1">
        <div class="flex items-center gap-3 mb-1">
          <span class="px-2 py-0.5 text-[10px] font-black rounded uppercase tracking-widest bg-base-content text-base-100">
            Global Perspective
          </span>
          <span class="text-[10px] font-bold text-base-content/40 tracking-widest uppercase italic">Updated: 3s ago</span>
        </div>
        <h1 class="text-3xl font-black tracking-tighter flex items-center gap-3">
          Wireflow 全域运行态势
          <span class="flex h-2.5 w-2.5 rounded-full bg-primary animate-pulse"></span>
        </h1>
        <p class="text-[11px] font-medium text-base-content/60 italic">
          正在汇总 12 个工作空间的实时算力编排、全球隧道分布及边缘节点健康状况。
        </p>
      </div>

      <div class="flex gap-2">
        <button class="btn btn-sm btn-ghost border-base-content/10 font-black text-[10px] uppercase">导出报表</button>
        <button class="btn btn-sm btn-primary font-black text-[10px] uppercase">+ 新建空间</button>
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
      <div v-for="s in dashboardStore.data.global_stats" :key="s.label"
           class="bg-base-100 border border-base-content/10 p-5 rounded-2xl shadow-sm hover:border-primary/30 transition-all">
        <div class="flex justify-between items-start">
          <div class="text-[9px] font-black uppercase tracking-widest text-base-content/40">{{ s.label }}</div>
          <div class="text-[10px] font-black italic text-primary opacity-60">{{ s.trend }}</div>
        </div>
        <div class="mt-2 flex items-baseline gap-1">
          <span class="text-3xl font-black tracking-tighter" :class="s.color">{{ s.value }}</span>
          <span class="text-[10px] font-bold text-base-content/20 font-mono">{{ s.unit }}</span>
        </div>
        <div class="mt-4 h-1 w-full bg-base-300 rounded-full overflow-hidden">
          <div class="h-full bg-primary transition-all duration-1000" :class="s.barWidth"></div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 xl:grid-cols-3 gap-6">

      <div class="xl:col-span-2 bg-base-100 border border-base-content/10 rounded-2xl overflow-hidden shadow-sm flex flex-col">
        <div class="px-6 py-4 border-b border-base-content/5 flex items-center justify-between bg-base-200/30">
          <h2 class="text-xs font-black uppercase tracking-widest italic opacity-70 text-primary">Workspace Leaderboard</h2>
          <span class="text-[9px] font-black opacity-40 uppercase">Sort by: Nodes Count</span>
        </div>
        <div class="overflow-x-auto">
          <table class="table w-full">
            <thead>
            <tr class="text-[9px] font-black uppercase opacity-40 border-b border-base-content/5">
              <th class="pl-8">工作空间名称</th>
              <th>实例规模</th>
              <th>本月流量</th>
              <th>健康指数</th>
              <th class="text-right pr-8">管理</th>
            </tr>
            </thead>
            <tbody class="text-[12px] font-bold">
            <tr v-for="ws in dashboardStore.data.workspace_usage" :key="ws.name" class="hover:bg-base-200/50 transition-colors cursor-pointer group">
              <td class="pl-8 py-4">
                <div class="flex flex-col">
                  <span class="group-hover:text-primary transition-colors">{{ ws.name }}</span>
                  <span class="text-[9px] font-mono opacity-40">Production-Cluster</span>
                </div>
              </td>
              <td>
                <div class="flex items-center gap-2">
                  <span class="text-base-content">{{ ws.nodes }}</span>
                  <progress class="progress progress-primary w-12 h-1" :value="ws.nodes" max="500"></progress>
                </div>
              </td>
              <td class="opacity-60 italic">{{ ws.traffic }}</td>
              <td>
                <div class="flex items-center gap-1" :class="ws.health < 95 ? 'text-warning' : 'text-success'">
                  <span class="text-[10px] font-mono">{{ ws.health }}%</span>
                </div>
              </td>
              <td class="text-right pr-8">
                <button class="btn btn-ghost btn-xs text-primary font-black">进入监控</button>
              </td>
            </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="bg-base-100 border border-base-content/10 rounded-2xl flex flex-col shadow-sm overflow-hidden">
        <div class="px-5 py-3 border-b border-base-content/5 bg-error/5 flex justify-between items-center">
          <h2 class="text-[10px] font-black uppercase tracking-widest text-error">全域风险审计</h2>
          <span class="badge badge-error badge-xs animate-pulse"></span>
        </div>
        <div class="divide-y divide-base-content/5 overflow-y-auto">
          <div v-for="(e, i) in dashboardStore.data.global_events" :key="i" class="p-4 space-y-2 hover:bg-base-200/50 transition-all group">
            <div class="flex justify-between items-center">
              <span class="text-[9px] font-mono font-bold text-base-content/30">{{ e.time }}</span>
              <span class="text-[8px] font-black text-primary px-1 border border-primary/20 rounded uppercase">{{ e.ws }}</span>
            </div>
            <div class="flex gap-2 items-start">
              <div :class="['w-1 h-3 rounded-full mt-0.5', toneStyles[e.tone]]"></div>
              <p class="text-[11px] font-bold text-base-content/70 group-hover:text-base-content leading-tight">
                {{ e.content }}
              </p>
            </div>
          </div>
        </div>
        <div class="p-4 mt-auto border-t border-base-content/5">
          <button class="btn btn-block btn-ghost btn-xs font-black opacity-40 hover:opacity-100 uppercase">查看全域日志历史</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.animate-fade-in { animation: fadeIn 0.4s ease-out forwards; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }
</style>