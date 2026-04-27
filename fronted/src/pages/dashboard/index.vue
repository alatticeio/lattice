<script setup lang="ts">
import { computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Activity, Server, ShieldCheck, AlertTriangle,
  ArrowUpRight, ArrowDownRight, MoreHorizontal, Globe, Building2, RefreshCw,
} from 'lucide-vue-next'
import { useDashboardStore } from '@/stores/useDashboard'
import { useWorkspaceStore } from '@/stores/workspace'

definePage({
  meta: {
    titleKey: 'settings.dashboard.title',
    descKey:  'settings.dashboard.desc',
  },
})

const { t } = useI18n()

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
const modeLabel      = computed(() => store.isWorkspaceMode ? t('settings.dashboard.workspaceMode') : t('settings.dashboard.globalMode'))
const modeIcon       = computed(() => store.isWorkspaceMode ? Building2 : Globe)
const throughputUnit = computed(() => store.isWorkspaceMode ? 'Mbps' : 'Gbps')

// ── connection quality cards ──────────────────────────────────────────
const qualityMetrics = computed(() => {
  const c = store.wsData?.stat_cards
  return [
    { label: 'Online Nodes', value: c?.[0]?.value ?? '—', unit: c?.[0]?.unit ?? '',  icon: Server,      cls: 'text-primary/50' },
    { label: 'Avg Latency',  value: c?.[2]?.value ?? '—', unit: c?.[2]?.unit ?? '',  icon: Activity,    cls: 'text-amber-500/50' },
    { label: 'Packet Loss',  value: c?.[3]?.value ?? '—', unit: c?.[3]?.unit ?? '',  icon: ShieldCheck, cls: 'text-emerald-500/50' },
  ]
})

// ── cpu bars with memory ──────────────────────────────────────────────
const cpuBars = computed(() => {
  const nodes = store.wsData?.node_cpu
  if (nodes?.length) {
    return [...nodes]
      .sort((a, b) => b.cpu - a.cpu)
      .slice(0, 5)
      .map(n => ({
        name:  n.name || n.peer_id,
        cpu:   Math.round(n.cpu),
        memMB: Math.round(n.memory_mb),
      }))
  }
  return store.nodeLoadBar.map(n => ({ name: n.name, cpu: n.load, memMB: 0 }))
})

// normalize bars to the busiest node so even low-CPU clusters look good
const maxCpu = computed(() => Math.max(...cpuBars.value.map(n => n.cpu), 1))
const BAR_MAX_PX = 64 // px height when at 100% of maxCpu
</script>

<template>
  <div class="flex flex-col gap-5 p-6">

    <!-- ── Mode badge ──────────────────────────────────────────────── -->
    <div class="flex items-center justify-between gap-2">
      <div class="flex items-center gap-1.5 rounded-full border border-border bg-muted/50 px-3 py-1 text-xs font-medium text-muted-foreground">
        <component :is="modeIcon" class="size-3" />
        {{ t('settings.dashboard.viewLabel', { mode: modeLabel }) }}
        <span v-if="store.wsLoading" class="ml-1 size-1.5 rounded-full bg-amber-400 animate-pulse" />
      </div>
      <button
        class="flex items-center gap-1.5 rounded-full border border-border bg-muted/50 px-3 py-1 text-xs font-medium text-muted-foreground hover:bg-muted transition-colors"
        :disabled="store.loading || store.wsLoading"
        @click="store.fetch(); store.fetchWorkspace()"
      >
        <RefreshCw class="size-3" :class="(store.loading || store.wsLoading) && 'animate-spin'" />
        Refresh
      </button>
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
            <p class="text-muted-foreground text-sm">{{ t('settings.dashboard.throughputSub', { mode: modeLabel, unit: throughputUnit }) }}</p>
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
          <h3 class="font-semibold">Node CPU Load</h3>
          <p class="text-muted-foreground text-sm">CPU usage per node (%)</p>
        </div>
        <div class="flex items-end justify-around gap-3 px-2" style="height: 80px">
          <template v-if="cpuBars.length > 0">
            <div
              v-for="node in cpuBars"
              :key="node.name"
              class="flex w-7 flex-col items-center gap-0.5"
            >
              <span class="text-muted-foreground text-[10px] tabular-nums">{{ node.cpu }}%</span>
              <div
                class="w-full rounded-t transition-all duration-700 bg-primary/60"
                :style="{ height: `${Math.max(Math.round(node.cpu / maxCpu * BAR_MAX_PX), 3)}px` }"
              />
            </div>
          </template>
          <template v-else>
            <div v-for="i in 5" :key="i" class="flex w-7 items-end">
              <div class="bg-muted rounded-t w-full animate-pulse" style="height: 40px" />
            </div>
          </template>
        </div>
        <!-- node name + memory row -->
        <div v-if="cpuBars.length > 0" class="flex gap-2 mt-1">
          <div
            v-for="node in cpuBars"
            :key="node.name"
            class="flex-1 flex flex-col items-center gap-0.5"
          >
            <span class="text-[9px] text-muted-foreground truncate w-full text-center leading-none">
              {{ node.name.length > 8 ? node.name.slice(0, 8) + '…' : node.name }}
            </span>
            <span v-if="node.memMB > 0" class="text-[9px] text-muted-foreground/60 leading-none">
              {{ node.memMB >= 1024 ? (node.memMB / 1024).toFixed(1) + 'G' : node.memMB + 'M' }}
            </span>
          </div>
        </div>
        <div class="mt-2 border-t border-border pt-3 text-xs text-muted-foreground">
          {{ cpuBars.length > 0 ? `${cpuBars.length} nodes monitored` : t('settings.dashboard.noNodeData') }}
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
                  {{ t('settings.dashboard.noNodeData') }}
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

      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 flex flex-col gap-3">
        <h3 class="font-semibold">Connection Quality</h3>

        <div
          v-for="m in qualityMetrics"
          :key="m.label"
          class="flex items-center justify-between rounded-lg bg-muted/40 px-4 py-3"
        >
          <div>
            <p class="text-[11px] text-muted-foreground uppercase tracking-wide font-medium">{{ m.label }}</p>
            <p class="text-xl font-bold mt-0.5 leading-none">
              <template v-if="store.loading || store.wsLoading">—</template>
              <template v-else>
                {{ m.value }}<span class="text-xs font-normal text-muted-foreground ml-1">{{ m.unit }}</span>
              </template>
            </p>
          </div>
          <component :is="m.icon" class="size-7 shrink-0" :class="m.cls" />
        </div>
      </div>
    </div>

  </div>
</template>
