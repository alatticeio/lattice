<script setup lang="ts">
import { ref, computed,watch, onMounted, onUnmounted} from 'vue'
import { useRoute } from 'vue-router'
import * as echarts from 'echarts'
// 假设你已经有了一个通用的 Icon 组件
import Icon from '@/components/icons/Icon.vue'
import { useMonitorStore } from '@/stores/monitor'
import { useWorkspaceStore} from '@/stores/workspace'
const monitorStore = useMonitorStore()
const workspaceStore = useWorkspaceStore()

const chartRef = ref<HTMLElement | null>(null)
let myChart: echarts.ECharts | null = null

// 初始化图表样式
const initChart = () => {
  if (!chartRef.value) return
  myChart = echarts.init(chartRef.value)
  myChart.setOption({
    grid: { left: '3%', right: '4%', bottom: '3%', top: '10%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, axisLabel: { color: '#94a3b8', fontSize: 10 } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: 'rgba(148,163,184,0.05)' } } },
    series: [
      { name: 'TX', type: 'line', smooth: true, symbol: 'none', color: '#3b82f6', areaStyle: { opacity: 0.1 }, data: [] },
      { name: 'RX', type: 'line', smooth: true, symbol: 'none', color: '#818cf8', areaStyle: { opacity: 0.1 }, data: [] }
    ]
  })
}

// 1. 实时轮询逻辑
let timer: any = null
const startPolling = () => {
  stopPolling()
  monitorStore.refresh()
  timer = setInterval(() => {
    monitorStore.refresh()
  }, 5000) // 5秒步长与后端 VM 查询步长匹配
}

const stopPolling = () => {
  if (timer) clearInterval(timer)
}


watch(() => monitorStore.data.trend, (newTrend) => {
  myChart?.setOption({
    xAxis: { data: newTrend.timestamps },
    series: [{ data: newTrend.tx_data }, { data: newTrend.rx_data }]
  })
}, { deep: true })

// 切换空间时立即刷新
watch(() => workspaceStore.activeId, startPolling)

onMounted(() => {
  initChart()
  window.addEventListener('resize', () => myChart?.resize())

  // 启动 Store 轮询
  monitorStore.refresh()
  timer = setInterval(() => monitorStore.refresh(), 2000)
})

onUnmounted(() => {
  clearInterval(timer)
  myChart?.dispose()
})

// 2. 辅助函数：根据后端返回的 Tone 匹配 Tailwind 类名
const getToneClass = (tone: string) => {
  const map: Record<string, string> = {
    emerald: 'bg-emerald-500/10 text-emerald-600 border-emerald-500/20',
    amber: 'bg-amber-500/10 text-amber-600 border-amber-500/20',
    blue: 'bg-blue-500/10 text-blue-600 border-blue-500/20',
  }
  return map[tone] || 'bg-slate-500/10 text-slate-600'
}

const route = useRoute()
const wsId = computed(() => route.params.wsId as string || 'default-workspace')


</script>

<template>
  <div class="max-w-[1400px] mx-auto p-4 lg:p-6 space-y-5 animate-fade-in font-sans">

    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 border-b border-slate-200 dark:border-white/5 pb-5">
      <div class="flex items-center gap-4">
        <div class="p-2.5 bg-blue-600 rounded-xl text-white shadow-lg shadow-blue-500/20">
          <Icon name="activity" class="w-5 h-5" />
        </div>
        <div>
          <h1 class="text-xl font-black tracking-tighter text-base-content dark:text-white flex items-center gap-2">
            实时态势监控
            <span class="text-[10px] px-2 py-0.5 bg-slate-100 dark:bg-base-100/5 rounded text-base-content/40 font-mono">WS: {{ wsId }}</span>
          </h1>
          <div class="flex items-center gap-2 mt-0.5">
            <span class="flex h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
            <span class="text-[9px] font-black text-base-content/40 uppercase tracking-[0.2em]">Real-time Data Stream Active</span>
          </div>
        </div>
      </div>
      <div class="flex items-center gap-2 bg-slate-100 dark:bg-base-100/5 p-1 rounded-xl border border-slate-200/50">
        <button class="px-4 py-1.5 text-[10px] font-black rounded-lg transition-all bg-base-100 dark:bg-slate-800 shadow-sm text-blue-600">实时流</button>
        <button class="px-4 py-1.5 text-[10px] font-black rounded-lg transition-all text-base-content/40 hover:text-base-content/70">历史回溯</button>
      </div>
    </div>

    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      <div v-for="s in monitorStore.data.live_stats" :key="s.label"
           class="bg-base-100 dark:bg-slate-900 border border-slate-200/60 dark:border-white/5 p-5 rounded-2xl shadow-sm hover:shadow-md transition-all group">
        <div class="text-[9px] font-black uppercase text-base-content/40 tracking-widest mb-2 flex justify-between">
          {{ s.label }}
          <Icon :name="s.trend" class="w-3 h-3" :class="s.trend === 'up' ? 'text-blue-500' : 'text-emerald-500'" />
        </div>
        <div class="flex items-baseline gap-1.5">
          <span class="text-3xl font-black tracking-tighter" :class="s.color">{{ s.value }}</span>
          <span class="text-[10px] font-bold opacity-30 font-mono uppercase">{{ s.unit }}</span>
        </div>
        <div class="mt-4 h-1 w-full bg-slate-100 dark:bg-base-100/5 rounded-full overflow-hidden">
          <div class="h-full bg-primary opacity-40 animate-progress" :style="{ width: '70%' }"></div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 xl:grid-cols-3 gap-5">
      <div class="xl:col-span-2 bg-base-100 dark:bg-slate-900 border border-slate-200/60 rounded-2xl overflow-hidden min-h-[280px] flex flex-col">
        <div class="px-5 py-3 border-b border-slate-50 dark:border-white/5 flex justify-between items-center">
          <span class="text-[10px] font-black uppercase tracking-widest text-base-content/40 italic">全域吞吐量走势 (Throughput)</span>
          <div class="flex gap-3">
            <span class="flex items-center gap-1.5 text-[9px] font-bold text-blue-500"><span class="w-2 h-0.5 bg-primary"></span> TX</span>
            <span class="flex items-center gap-1.5 text-[9px] font-bold text-indigo-400"><span class="w-2 h-0.5 bg-indigo-400"></span> RX</span>
          </div>
        </div>
        <div class="flex-1 grid place-items-center bg-[radial-gradient(#e2e8f0_1px,transparent_1px)] dark:bg-[radial-gradient(#1e293b_1px,transparent_1px)] [background-size:20px_20px]">
<!--          <span class="text-[10px] font-bold text-slate-300 uppercase tracking-widest animate-pulse"></span>-->
          <div ref="chartRef" class="w-full h-full min-h-[240px]"></div>
        </div>
      </div>

      <div class="bg-base-100 dark:bg-slate-900 border border-slate-200/60 rounded-2xl flex flex-col">
        <div class="px-5 py-3 border-b border-slate-50 dark:border-white/5">
          <span class="text-[10px] font-black uppercase tracking-widest text-base-content/40">链路质量排行 (Latency)</span>
        </div>
        <div class="p-5 space-y-5 flex-1">
          <div v-for="i in monitorStore.data.latency_rank" :key="i.name" class="space-y-2">
            <div class="flex justify-between text-[11px] font-bold italic">
              <span class="text-base-content/60">{{i.name}} <Icon name="arrow-right" class="w-2 h-2 inline mx-1 opacity-20"/> {{i.target }}</span>
              <span :class="i.status">{{ 18 + i.percent * 12 }}ms</span>
            </div>
            <div class="w-full h-1 bg-slate-100 dark:bg-base-100/5 rounded-full overflow-hidden">
              <div class="h-full bg-primary transition-all duration-1000" :style="{ width: (25 + i.percent * 20) + '%' }"></div>
            </div>
          </div>
          <div class="mt-4 p-4 bg-slate-50 dark:bg-base-100/5 rounded-xl border border-dashed border-slate-200 dark:border-white/10">
            <span class="text-[9px] font-black text-blue-500 uppercase">智能诊断</span>
            <p class="text-[10px] text-base-content/60 mt-1 leading-relaxed">当前工作空间内 80% 的节点已通过 P2P 成功握手，平均链路质量优于全球 92% 的用户。</p>
          </div>
        </div>
      </div>
    </div>

    <div class="bg-base-100 dark:bg-slate-900 border border-slate-200/60 rounded-2xl overflow-hidden shadow-sm">
      <div class="px-6 py-4 border-b border-slate-50 dark:border-white/5 flex justify-between items-center bg-slate-50/30 dark:bg-base-100/[0.02]">
        <div class="flex items-center gap-3">
          <span class="text-[10px] font-black uppercase tracking-widest text-base-content/40">节点实时链路状态</span>
        </div>
        <div class="flex gap-4">
          <div class="flex items-center gap-1.5 text-[9px] font-bold text-emerald-500"><Icon name="zap" class="w-3 h-3"/> P2P</div>
          <div class="flex items-center gap-1.5 text-[9px] font-bold text-amber-500"><Icon name="globe" class="w-3 h-3"/> RELAY</div>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="table table-xs w-full">
          <thead>
          <tr class="text-base-content/40 border-b border-slate-50 dark:border-white/5">
            <th class="pl-6 py-4 text-[9px] font-black uppercase">节点名称 / VIP</th>
            <th class="pl-6 py-4 text-[9px] font-black uppercase">Target名称</th>
            <th class="text-[9px] font-black uppercase">连接模式</th>
            <th class="text-[9px] font-black uppercase">最后握手</th>
            <th class="text-[9px] font-black uppercase">累计 RX/TX</th>
            <th class="text-[9px] font-black uppercase">当前速率</th>
            <th class="text-[9px] font-black uppercase text-right pr-6">状态</th>
          </tr>
          </thead>
          <tbody class="divide-y divide-slate-50 dark:divide-white/5">
          <tr v-for="n in monitorStore.data.nodes" :key="n.id" class="group hover:bg-slate-50/50 dark:hover:bg-base-100/[0.01] transition-colors">
            <td class="pl-6 py-4">
              <div class="flex flex-col">
                <span class="font-bold text-slate-700 dark:text-slate-200 text-[13px] tracking-tight">{{ n.name }}</span>
                <span class="text-[10px] font-mono text-base-content/40 mt-0.5">{{ n.vip }}</span>
              </div>
            </td>

            <td class="pl-6 py-4">
              <div class="flex flex-col">
                <span class="font-bold text-slate-700 dark:text-slate-200 text-[13px] tracking-tight">{{ n.peerName }}</span>
              </div>
            </td>

            <td>
              <div class="flex items-center gap-2">
                  <span :class="[
                    'px-2 py-0.5 rounded text-[9px] font-black uppercase border',
                    n.connectionType === 'p2p' ? 'bg-emerald-500/10 text-emerald-600 border-emerald-500/20' : 'bg-amber-500/10 text-amber-600 border-amber-500/20'
                  ]">
                    {{ n.connectionType }}
                  </span>
                <span class="text-[10px] font-mono text-slate-300 hidden lg:inline">{{ n.endpoint }}</span>
              </div>
            </td>
            <td class="font-bold text-[11px] text-base-content/70 dark:text-base-content/40 italic">
              {{ n.lastHandshake }}
            </td>
            <td>
              <div class="flex items-center gap-3">
                <div class="flex flex-col">
                  <span class="text-[8px] font-black text-slate-300 uppercase italic">RX</span>
                  <span class="text-[11px] font-mono font-bold text-base-content/70">{{ n.totalRx }}</span>
                </div>
                <div class="h-6 w-px bg-slate-100 dark:bg-base-100/5"></div>
                <div class="flex flex-col">
                  <span class="text-[8px] font-black text-slate-300 uppercase italic">TX</span>
                  <span class="text-[11px] font-mono font-bold text-base-content/70">{{ n.totalTx }}</span>
                </div>
              </div>
            </td>
            <td>
              <div class="flex items-center gap-3">
                <div class="w-10 h-5 bg-slate-50 dark:bg-base-100/5 rounded flex items-end gap-0.5 p-0.5 overflow-hidden">
                  <div v-for="b in 4" :key="b" class="flex-1 bg-primary/30 rounded-t-[1px] animate-pulse" :style="{ height: Math.random()*100+'%' }"></div>
                </div>
                <span class="text-[11px] font-black text-blue-600">{{ n.currentRate }}</span>
              </div>
            </td>
            <td class="text-right pr-6">
                <span :class="['px-2 py-0.5 rounded text-[9px] font-black uppercase', n.online ? 'bg-emerald-500 text-white' : 'bg-slate-200 text-base-content/40']">
                  {{ n.online ? 'Active' : 'Offline' }}
                </span>
            </td>
          </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="bg-[#0f172a] rounded-2xl border border-white/10 overflow-hidden shadow-2xl">
      <div class="px-5 py-2.5 border-b border-white/5 flex justify-between items-center bg-white/5">
        <span class="text-[10px] font-black text-slate-400 uppercase tracking-[0.2em]">Workspace Event Stream</span>
        <button class="text-[10px] font-black text-blue-400 uppercase hover:text-blue-300 transition-colors">Full Logs</button>
      </div>

      <div class="p-1 max-h-[220px] overflow-y-auto custom-scrollbar">
        <div v-for="(log, i) in monitorStore.data.events" :key="i"
             class="flex items-center gap-3 px-4 py-1 hover:bg-white/[0.05] transition-all rounded-md">

          <span class="text-[10px] font-mono text-slate-500 shrink-0 w-[60px]">{{ log.time }}</span>

          <span :class="[
        'text-[9px] font-black px-1.5 py-0.5 rounded uppercase shrink-0 min-w-[36px] text-center',
        log.level === 'info' ? 'text-blue-400 bg-blue-500/10' : 'text-amber-400 bg-amber-500/10'
      ]">
        {{ log.level }}
      </span>

          <span class="text-[12px] text-white font-medium tracking-tight leading-tight truncate flex-1">
        {{ log.msg }}
      </span>
        </div>
      </div>
    </div>

  </div>
</template>

<style scoped>
.animate-fade-in { animation: fadeIn 0.5s ease-out; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
@keyframes progress { from { transform: translateX(-100%); } to { transform: translateX(100%); } }
.animate-progress { animation: progress 2s infinite linear; }
</style>