<script setup lang="ts">
import { computed, watch, onMounted, onUnmounted } from 'vue'
import {
  Activity, Server, ShieldCheck, AlertTriangle,
  ArrowUpRight, ArrowDownRight, Zap, MoreHorizontal, Globe, Building2,
} from 'lucide-vue-next'
import { useDashboardStore } from '@/stores/useDashboard'
import { useWorkspaceStore } from '@/stores/workspace'

definePage({
  meta: {
    title: 'Wireflow Dashboard',
    description: '全域网络态势实时监控中。',
  },
})

const store = useDashboardStore()
onMounted(() => store.startPolling())
onUnmounted(() => store.stopPolling())

const workspaceStore = useWorkspaceStore()
watch(() => workspaceStore.currentWorkspace?.id, (newId, oldId) => {
  if (newId !== oldId) {
    store.fetch()
    store.fetchWorkspace()
  }
})

// ── colour mapping for audit log dots ────────────────────────────────
const toneMap: Record<string, string> = {
  emerald: 'bg-emerald-500',
  blue:    'bg-blue-500',
  amber:   'bg-amber-500',
  red:     'bg-red-500',
  rose:    'bg-rose-500',
}

// ── icon lookup for stat cards ────────────────────────────────────────
const iconByIndex = [Server, Activity, ShieldCheck, AlertTriangle]

// ── stat cards: workspace when active, global otherwise ──────────────
const stats = computed(() =>
  store.displayStatCards.map((s, i) => ({
    ...s,
    icon: iconByIndex[i] ?? Server,
  }))
)

// ── SVG path builder ──────────────────────────────────────────────────
function buildPath(data: number[], w: number, h: number, pad = 8) {
  const safeData = data.length > 1 ? data : [0, 1]
  const max = Math.max(...safeData)
  const min = Math.min(...safeData)
  const range = max - min || 1
  const xStep = (w - pad * 2) / (safeData.length - 1)
  const pts = safeData.map((v, i) => ({
    x: pad + i * xStep,
    y: h - pad - ((v - min) / range) * (h - pad * 2),
  }))
  const line = pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')
  const area = `${line} L${pts.at(-1)!.x.toFixed(1)},${h - pad} L${pts[0].x.toFixed(1)},${h - pad} Z`
  return { line, area, pts }
}

// ── throughput chart ──────────────────────────────────────────────────
const upChart   = computed(() => buildPath(store.txChartData, 520, 180, 16))
const downChart = computed(() => buildPath(store.rxChartData, 520, 180, 16))

// ── mode label ────────────────────────────────────────────────────────
const modeLabel = computed(() => store.isWorkspaceMode ? '工作空间' : '全域')
const modeIcon  = computed(() => store.isWorkspaceMode ? Building2 : Globe)
const throughputUnit = computed(() => store.isWorkspaceMode ? 'Mbps' : 'Gbps')
</script>

<template>
  <div class="flex flex-col gap-5 p-6">

    <!-- ── Mode badge ──────────────────────────────────────────────── -->
    <div class="flex items-center gap-2">
      <div class="flex items-center gap-1.5 rounded-full border border-border bg-muted/50 px-3 py-1 text-xs font-medium text-muted-foreground">
        <component :is="modeIcon" class="size-3" />
        {{ modeLabel }}视图
        <span v-if="store.wsLoading" class="ml-1 size-1.5 rounded-full bg-amber-400 animate-pulse" />
      </div>
    </div>

    <!-- ── Stat Cards ──────────────────────────────────────────────── -->
    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div
        v-for="(stat, i) in stats"
        :key="i"
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ stat.title }}</span>
            <span class="text-2xl font-bold tracking-tight">
              <template v-if="store.loading || store.wsLoading">—</template>
              <template v-else>{{ stat.value }}</template>
            </span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <component :is="stat.icon" class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <component
            :is="stat.trend === 'up' ? ArrowUpRight : ArrowDownRight"
            :class="stat.trend === 'up' ? 'text-emerald-600' : 'text-red-500'"
            class="size-4"
          />
          <span :class="stat.trend === 'up' ? 'text-emerald-600' : 'text-red-500'" class="font-semibold">
            {{ stat.change }}
          </span>
        </div>
        <svg class="mt-3 w-full" viewBox="0 0 80 28" preserveAspectRatio="none" style="height:28px">
          <path
            :d="buildPath(stat.sparkline, 80, 28, 2).line"
            fill="none"
            :stroke="stat.trend === 'up' ? '#10b981' : '#ef4444'"
            stroke-width="1.5"
            stroke-linecap="round"
          />
        </svg>
      </div>

      <!-- skeleton when no data yet -->
      <template v-if="stats.length === 0">
        <div
          v-for="i in 4"
          :key="i"
          class="border-border bg-card rounded-xl border p-5 shadow-sm animate-pulse"
        >
          <div class="h-4 w-24 bg-muted rounded mb-3" />
          <div class="h-8 w-20 bg-muted rounded mb-3" />
          <div class="h-3 w-32 bg-muted rounded" />
        </div>
      </template>
    </div>

    <!-- ── Throughput Chart + Node Load ───────────────────────────── -->
    <div class="grid gap-4 lg:grid-cols-3">
      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 lg:col-span-2">
        <div class="mb-4 flex items-start justify-between">
          <div>
            <h3 class="font-semibold">Network Throughput</h3>
            <p class="text-muted-foreground text-sm">{{ modeLabel }}实时流量监控 ({{ throughputUnit }})</p>
          </div>
          <div class="flex items-center gap-4 text-xs font-medium">
            <div class="flex items-center gap-1.5">
              <span class="size-2.5 rounded-full bg-primary" /> Outbound TX
            </div>
            <div class="flex items-center gap-1.5">
              <span class="size-2.5 rounded-full bg-blue-400" /> Inbound RX
            </div>
          </div>
        </div>
        <svg viewBox="0 0 520 180" class="w-full" style="height:180px">
          <defs>
            <linearGradient id="upGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="var(--primary)" stop-opacity="0.3" />
              <stop offset="100%" stop-color="var(--primary)" stop-opacity="0" />
            </linearGradient>
          </defs>
          <line v-for="i in 4" :key="i" :y1="i * 40" :y2="i * 40" x1="16" x2="504"
                stroke="currentColor" stroke-opacity="0.08" />
          <path :d="downChart.line" fill="none" stroke="#60a5fa" stroke-width="1.5" stroke-dasharray="4 3" />
          <path :d="upChart.area" fill="url(#upGrad)" />
          <path :d="upChart.line" fill="none" stroke="var(--primary)" stroke-width="2" />
        </svg>
        <div class="mt-1 flex justify-between px-4 text-xs text-muted-foreground">
          <span v-for="t in store.chartTimeline" :key="t">{{ t }}</span>
        </div>
      </div>

      <div class="border-border bg-card text-card-foreground rounded-xl border p-5">
        <div class="mb-4">
          <h3 class="font-semibold">Node Load</h3>
          <p class="text-muted-foreground text-sm">当前节点 CPU 负载分布</p>
        </div>
        <div class="flex h-40 items-end gap-2">
          <template v-if="store.nodeLoadBar.length > 0">
            <div
              v-for="node in store.nodeLoadBar"
              :key="node.name"
              class="flex flex-1 flex-col items-center gap-1"
            >
              <span class="text-muted-foreground text-[10px] font-medium">{{ node.load }}%</span>
              <div
                class="w-full rounded-t transition-all duration-700"
                :class="node.load > 80 ? 'bg-red-500/80' : node.load > 60 ? 'bg-amber-400/80' : 'bg-primary/80'"
                :style="{ height: `${Math.max(node.load, 4)}%` }"
              />
            </div>
          </template>
          <template v-else>
            <div v-for="i in 5" :key="i" class="flex flex-1 flex-col items-center gap-1">
              <div class="bg-muted rounded h-4 w-full animate-pulse" />
            </div>
          </template>
        </div>
        <div class="mt-4 border-t border-border pt-4 flex items-center justify-between">
          <div class="flex items-center gap-2 text-primary font-semibold text-sm">
            <Zap class="size-4" /> 加速引擎活动中
          </div>
          <span class="text-xs text-muted-foreground italic">Optimal</span>
        </div>
      </div>
    </div>

    <!-- ── High-Traffic Nodes + Audit Logs ───────────────────────── -->
    <div class="grid gap-4 lg:grid-cols-3">
      <div class="border-border bg-card text-card-foreground rounded-xl border lg:col-span-2 overflow-hidden">
        <div class="border-b border-border p-5 flex justify-between items-center">
          <h3 class="font-semibold text-sm">High-Traffic Nodes</h3>
          <button class="text-muted-foreground hover:text-foreground">
            <MoreHorizontal class="size-4" />
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-border bg-muted/30">
                <th class="px-5 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Node</th>
                <th class="px-5 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Endpoint</th>
                <th class="px-5 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Traffic (24h)</th>
                <th class="px-5 py-3 text-right text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              <tr
                v-for="node in store.topTrafficNodes"
                :key="node.name"
                class="hover:bg-muted/30 transition-colors"
              >
                <td class="px-5 py-3 font-medium truncate max-w-[160px]">{{ node.name }}</td>
                <td class="px-5 py-3 text-muted-foreground font-mono text-xs">{{ node.ip }}</td>
                <td class="px-5 py-3 font-semibold">{{ node.traffic }}</td>
                <td class="px-5 py-3 text-right">
                  <span
                    class="px-2.5 py-0.5 rounded-full text-xs font-medium"
                    :class="node.status === 'Healthy'
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30'
                      : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30'"
                  >
                    {{ node.status }}
                  </span>
                </td>
              </tr>
              <tr v-if="store.topTrafficNodes.length === 0 && !store.loading">
                <td colspan="4" class="px-5 py-6 text-center text-muted-foreground text-sm">
                  暂无节点数据 — 请确认监控服务已就绪
                </td>
              </tr>
              <tr v-if="store.loading">
                <td colspan="4">
                  <div class="flex flex-col gap-2 p-4">
                    <div v-for="i in 3" :key="i" class="h-4 bg-muted rounded animate-pulse" />
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 overflow-hidden flex flex-col">
        <h3 class="font-semibold mb-4">Audit Logs</h3>
        <div class="space-y-4 flex-1">
          <div v-for="(log, i) in store.auditLogs" :key="i" class="flex items-center gap-3">
            <div :class="[toneMap[log.tone] ?? 'bg-blue-500', 'size-2 rounded-full shadow-sm shrink-0']" />
            <div class="flex-1 min-w-0">
              <p class="text-xs font-medium truncate">{{ log.action }}</p>
              <p class="text-[10px] text-muted-foreground">{{ log.time }} · {{ log.user }}</p>
            </div>
            <div class="text-[10px] text-muted-foreground italic truncate max-w-[80px]">{{ log.target }}</div>
          </div>
          <p v-if="store.auditLogs.length === 0 && !store.loading"
             class="text-xs text-muted-foreground text-center py-4">
            暂无日志
          </p>
        </div>
        <button class="mt-4 w-full py-2 border border-border rounded-md text-xs font-medium hover:bg-muted transition-colors">
          View All Logs
        </button>
      </div>
    </div>

  </div>
</template>
