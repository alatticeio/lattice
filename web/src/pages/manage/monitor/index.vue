<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Activity, Cpu, Network, Clock, ArrowUpDown } from 'lucide-vue-next'

definePage({
  meta: { title: '监控', description: '实时网络流量与节点状态监控。' },
})

// --- Live stats ---
const stats = ref({ txRate: 0, rxRate: 0, onlineNodes: 0, avgLatency: 0 })

// --- Throughput chart data (last 30 samples) ---
const MAX_POINTS = 30
const txHistory = ref<number[]>(Array(MAX_POINTS).fill(0))
const rxHistory = ref<number[]>(Array(MAX_POINTS).fill(0))

// --- Node links table ---
interface NodeLink {
  peer: string
  connectionType: 'p2p' | 'relay'
  lastHandshake: string
  totalRx: string
  totalTx: string
  currentRate: string
}

const links = ref<NodeLink[]>([
  { peer: 'node-alpha', connectionType: 'p2p', lastHandshake: '5s ago', totalRx: '1.2 GB', totalTx: '0.8 GB', currentRate: '2.4 MB/s' },
  { peer: 'node-beta', connectionType: 'relay', lastHandshake: '12s ago', totalRx: '340 MB', totalTx: '120 MB', currentRate: '0.8 MB/s' },
  { peer: 'node-epsilon', connectionType: 'p2p', lastHandshake: '2s ago', totalRx: '4.1 GB', totalTx: '3.7 GB', currentRate: '5.1 MB/s' },
  { peer: 'node-zeta', connectionType: 'relay', lastHandshake: '45s ago', totalRx: '78 MB', totalTx: '22 MB', currentRate: '0.1 MB/s' },
])

// --- Event log ---
const logs = ref<string[]>([
  '[12:04:32] node-alpha handshake complete (p2p)',
  '[12:04:28] node-zeta reconnected via relay',
  '[12:04:15] node-beta latency spike detected: 240ms',
  '[12:03:58] node-epsilon established p2p connection',
  '[12:03:42] node-gamma disconnected',
  '[12:03:30] system: 监控服务已启动',
])

// --- Latency ranking ---
const latencyRanking = ref([
  { name: 'node-alpha', ms: 12, pct: 12 },
  { name: 'node-epsilon', ms: 18, pct: 18 },
  { name: 'node-beta', ms: 45, pct: 45 },
  { name: 'node-zeta', ms: 98, pct: 98 },
])

let timer: ReturnType<typeof setInterval>

function tick() {
  const tx = Math.random() * 8 + 1
  const rx = Math.random() * 6 + 0.5
  txHistory.value.push(tx)
  txHistory.value.shift()
  rxHistory.value.push(rx)
  rxHistory.value.shift()

  stats.value = {
    txRate: tx,
    rxRate: rx,
    onlineNodes: Math.floor(Math.random() * 3) + 3,
    avgLatency: Math.floor(Math.random() * 40 + 20),
  }

  // add a log entry occasionally
  if (Math.random() > 0.6) {
    const peers = ['node-alpha', 'node-beta', 'node-epsilon']
    const peer = peers[Math.floor(Math.random() * peers.length)]
    const now = new Date()
    const ts = `${String(now.getHours()).padStart(2,'0')}:${String(now.getMinutes()).padStart(2,'0')}:${String(now.getSeconds()).padStart(2,'0')}`
    logs.value.unshift(`[${ts}] ${peer}: rate ${(Math.random() * 5 + 0.5).toFixed(1)} MB/s`)
    if (logs.value.length > 50) logs.value.pop()
  }
}

onMounted(() => { tick(); timer = setInterval(tick, 3000) })
onUnmounted(() => clearInterval(timer))

// SVG chart helpers
const W = 480
const H = 120

function buildPath(data: number[]): string {
  const max = Math.max(...data, 1)
  const pts = data.map((v, i) => {
    const x = (i / (data.length - 1)) * W
    const y = H - (v / max) * H
    return `${x},${y}`
  })
  return `M ${pts.join(' L ')}`
}

function buildArea(data: number[]): string {
  const max = Math.max(...data, 1)
  const pts = data.map((v, i) => {
    const x = (i / (data.length - 1)) * W
    const y = H - (v / max) * H
    return `${x},${y}`
  })
  return `M 0,${H} L ${pts.join(' L ')} L ${W},${H} Z`
}
</script>

<template>
  <div class="p-6 space-y-6">
    <!-- Stats cards -->
    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div class="bg-card border border-border rounded-xl p-5 flex items-center gap-4">
        <div class="size-10 rounded-lg bg-primary/10 flex items-center justify-center">
          <ArrowUpDown class="size-5 text-primary" />
        </div>
        <div>
          <p class="text-xs text-muted-foreground uppercase tracking-wider">上行速率</p>
          <p class="text-xl font-bold">{{ stats.txRate.toFixed(1) }} <span class="text-sm font-normal text-muted-foreground">MB/s</span></p>
        </div>
      </div>
      <div class="bg-card border border-border rounded-xl p-5 flex items-center gap-4">
        <div class="size-10 rounded-lg bg-sky-500/10 flex items-center justify-center">
          <Network class="size-5 text-sky-500" />
        </div>
        <div>
          <p class="text-xs text-muted-foreground uppercase tracking-wider">下行速率</p>
          <p class="text-xl font-bold">{{ stats.rxRate.toFixed(1) }} <span class="text-sm font-normal text-muted-foreground">MB/s</span></p>
        </div>
      </div>
      <div class="bg-card border border-border rounded-xl p-5 flex items-center gap-4">
        <div class="size-10 rounded-lg bg-emerald-500/10 flex items-center justify-center">
          <Cpu class="size-5 text-emerald-500" />
        </div>
        <div>
          <p class="text-xs text-muted-foreground uppercase tracking-wider">在线节点</p>
          <p class="text-xl font-bold">{{ stats.onlineNodes }}</p>
        </div>
      </div>
      <div class="bg-card border border-border rounded-xl p-5 flex items-center gap-4">
        <div class="size-10 rounded-lg bg-amber-500/10 flex items-center justify-center">
          <Clock class="size-5 text-amber-500" />
        </div>
        <div>
          <p class="text-xs text-muted-foreground uppercase tracking-wider">平均延迟</p>
          <p class="text-xl font-bold">{{ stats.avgLatency }} <span class="text-sm font-normal text-muted-foreground">ms</span></p>
        </div>
      </div>
    </div>

    <!-- Throughput chart + Latency ranking -->
    <div class="grid gap-4 lg:grid-cols-3">
      <!-- Chart -->
      <div class="lg:col-span-2 bg-card border border-border rounded-xl p-5">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-sm font-semibold">实时吞吐量</h3>
          <div class="flex items-center gap-4 text-xs text-muted-foreground">
            <span class="flex items-center gap-1.5">
              <span class="size-2 rounded-full bg-primary inline-block" />上行 TX
            </span>
            <span class="flex items-center gap-1.5">
              <span class="size-2 rounded-full bg-sky-500 inline-block" />下行 RX
            </span>
          </div>
        </div>
        <svg :viewBox="`0 0 ${W} ${H}`" class="w-full" style="height: 140px" preserveAspectRatio="none">
          <defs>
            <linearGradient id="txGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="var(--primary)" stop-opacity="0.3" />
              <stop offset="100%" stop-color="var(--primary)" stop-opacity="0" />
            </linearGradient>
            <linearGradient id="rxGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="oklch(0.6 0.2 220)" stop-opacity="0.3" />
              <stop offset="100%" stop-color="oklch(0.6 0.2 220)" stop-opacity="0" />
            </linearGradient>
          </defs>
          <!-- TX area -->
          <path :d="buildArea(txHistory)" fill="url(#txGrad)" />
          <path :d="buildPath(txHistory)" fill="none" stroke="var(--primary)" stroke-width="1.5" />
          <!-- RX area -->
          <path :d="buildArea(rxHistory)" fill="url(#rxGrad)" />
          <path :d="buildPath(rxHistory)" fill="none" stroke="oklch(0.6 0.2 220)" stroke-width="1.5" stroke-dasharray="4 2" />
        </svg>
        <p class="text-xs text-muted-foreground mt-2 text-right">每 3 秒刷新</p>
      </div>

      <!-- Latency ranking -->
      <div class="bg-card border border-border rounded-xl p-5">
        <h3 class="text-sm font-semibold mb-4">延迟排名</h3>
        <div class="space-y-3">
          <div v-for="(item, i) in latencyRanking" :key="item.name" class="space-y-1">
            <div class="flex justify-between text-xs">
              <span class="flex items-center gap-1.5">
                <span class="text-muted-foreground">#{{ i + 1 }}</span>
                <span class="font-medium">{{ item.name }}</span>
              </span>
              <span :class="item.ms > 80 ? 'text-rose-500' : item.ms > 40 ? 'text-amber-500' : 'text-emerald-500'" class="font-medium">
                {{ item.ms }} ms
              </span>
            </div>
            <div class="h-1.5 bg-muted rounded-full overflow-hidden">
              <div
                class="h-full rounded-full"
                :class="item.ms > 80 ? 'bg-rose-500' : item.ms > 40 ? 'bg-amber-500' : 'bg-emerald-500'"
                :style="{ width: item.pct + '%' }"
              />
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Node links table + Event log -->
    <div class="grid gap-4 lg:grid-cols-3">
      <!-- Links table -->
      <div class="lg:col-span-2 bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-5 py-4 border-b border-border">
          <h3 class="text-sm font-semibold">节点连接</h3>
        </div>
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-border bg-muted/30">
              <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">对端</th>
              <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">类型</th>
              <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground hidden sm:table-cell">握手</th>
              <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground hidden md:table-cell">总流量</th>
              <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">速率</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="link in links" :key="link.peer" class="border-b border-border last:border-0 hover:bg-muted/20 transition-colors">
              <td class="px-4 py-3 font-medium">{{ link.peer }}</td>
              <td class="px-4 py-3">
                <span
                  class="text-xs rounded-full px-2 py-0.5 font-medium"
                  :class="link.connectionType === 'p2p'
                    ? 'bg-primary/10 text-primary'
                    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'"
                >
                  {{ link.connectionType }}
                </span>
              </td>
              <td class="px-4 py-3 text-muted-foreground hidden sm:table-cell text-xs">{{ link.lastHandshake }}</td>
              <td class="px-4 py-3 text-muted-foreground hidden md:table-cell text-xs">
                ↑{{ link.totalTx }} ↓{{ link.totalRx }}
              </td>
              <td class="px-4 py-3 font-medium text-xs">{{ link.currentRate }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Event log -->
      <div class="bg-zinc-950 dark:bg-zinc-900 border border-zinc-800 rounded-xl overflow-hidden flex flex-col">
        <div class="px-4 py-3 border-b border-zinc-800 flex items-center gap-2">
          <Activity class="size-3.5 text-emerald-400" />
          <h3 class="text-xs font-semibold text-zinc-300">事件日志</h3>
          <span class="ml-auto size-2 rounded-full bg-emerald-500 animate-pulse" />
        </div>
        <div class="flex-1 overflow-y-auto p-4 space-y-1 font-mono text-xs" style="max-height: 260px">
          <p v-for="(log, i) in logs" :key="i" class="text-emerald-400 leading-relaxed">{{ log }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
