<script setup lang="ts">
import { computed } from 'vue'

definePage({
  meta: {
    title: 'Wireflow Dashboard',
    description: "全域网络态势实时监控中。",
  },
})

import {
  Activity, Server, ShieldCheck, AlertTriangle,
  ArrowUpRight, ArrowDownRight, Zap,
} from 'lucide-vue-next'

// ── 1. 颜色映射表 (用于审计日志小圆点) ───────────────────────────────────
const toneMap: Record<string, string> = {
  emerald: 'bg-emerald-500',
  blue: 'bg-blue-500',
  amber: 'bg-amber-500',
  red: 'bg-red-500',
}

// ── 2. 核心指标数据 ───────────────────────────────────────────────────────
const stats = [
  { title: '在线节点', value: '128 / 132', change: '+3', trend: 'up' as const, description: '过去 24h 新增', icon: Server, sparkline: [120, 122, 121, 125, 124, 128, 126, 128, 127, 128, 128, 128] },
  { title: '总吞吐量', value: '4.2 Gbps', change: '+12.5%', trend: 'up' as const, description: '峰值带宽占用', icon: Activity, sparkline: [30, 45, 60, 40, 50, 75, 90, 85, 70, 95, 110, 100] },
  { title: '活跃策略', value: '42', change: '0', trend: 'up' as const, description: '全域生效中', icon: ShieldCheck, sparkline: [42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42] },
  { title: '系统告警', value: '3', change: '-2', trend: 'down' as const, description: '待处理风险', icon: AlertTriangle, sparkline: [8, 10, 5, 7, 3, 4, 6, 2, 4, 1, 3, 3] },
]

// ── 3. 绘图逻辑 (保持电商原版极简风格) ──────────────────────────────────
function buildPath(data: number[], w: number, h: number, pad = 8) {
  const max = Math.max(...data)
  const min = Math.min(...data)
  const range = max - min || 1
  const xStep = (w - pad * 2) / (data.length - 1)
  const pts = data.map((v, i) => ({
    x: pad + i * xStep,
    y: h - pad - ((v - min) / range) * (h - pad * 2),
  }))
  const line = pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')
  const area = `${line} L${pts.at(-1)!.x.toFixed(1)},${h - pad} L${pts[0].x.toFixed(1)},${h - pad} Z`
  return { line, area, pts }
}

const timeline = ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00']
const upChart = computed(() => buildPath([1.2, 1.5, 2.8, 3.2, 2.1, 4.2, 3.8, 4.5, 5.2, 4.8, 4.2, 4.5], 520, 180, 16))
const downChart = computed(() => buildPath([0.8, 1.1, 2.0, 2.5, 1.8, 3.0, 2.5, 3.2, 4.0, 3.5, 3.0, 3.2], 520, 180, 16))

const topNodes = [
  { name: 'AWS-Virginia-Edge', ip: '54.12.8.11', traffic: '1.2 TB', load: 82, status: 'Healthy' },
  { name: 'HK-CN2-GIA', ip: '43.22.10.5', traffic: '942 GB', load: 65, status: 'Healthy' },
  { name: 'Linode-Tokyo', ip: '139.16.2.88', traffic: '850 GB', load: 42, status: 'Warning' },
  { name: 'DigitalOcean-SG', ip: '159.2.4.1', traffic: '720 GB', load: 30, status: 'Healthy' },
  { name: 'Hetzner-Frankfurt', ip: '95.1.5.12', traffic: '610 GB', load: 15, status: 'Healthy' },
]

const auditLogs = [
  { time: '10:24', user: 'Admin', action: 'Update Policy', target: 'Global-ACL', tone: 'emerald' },
  { time: '10:15', user: 'System', action: 'Node Join', target: 'edge-node-99', tone: 'blue' },
  { time: '09:12', user: 'Security', action: 'Block IP', target: '192.168.1.100', tone: 'red' },
]
</script>

<template>
  <div class="flex flex-col gap-5 p-6">

    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div
          v-for="stat in stats"
          :key="stat.title"
          class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ stat.title }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stat.value }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <component :is="stat.icon" class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <component :is="stat.trend === 'up' ? ArrowUpRight : ArrowDownRight"
                     :class="stat.trend === 'up' ? 'text-emerald-600' : 'text-red-500'" class="size-4" />
          <span :class="stat.trend === 'up' ? 'text-emerald-600' : 'text-red-500'" class="font-semibold">{{ stat.change }}</span>
          <span class="text-muted-foreground">{{ stat.description }}</span>
        </div>
        <svg class="mt-3 w-full" viewBox="0 0 80 28" preserveAspectRatio="none" style="height:28px">
          <path :d="buildPath(stat.sparkline, 80, 28, 2).line" fill="none"
                :stroke="stat.trend === 'up' ? '#10b981' : '#ef4444'" stroke-width="1.5" stroke-linecap="round" />
        </svg>
      </div>
    </div>

    <div class="grid gap-4 lg:grid-cols-3">
      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 lg:col-span-2">
        <div class="mb-4 flex items-start justify-between">
          <div>
            <h3 class="font-semibold">Network Throughput</h3>
            <p class="text-muted-foreground text-sm">全域实时流量监控</p>
          </div>
          <div class="flex items-center gap-4 text-xs font-medium">
            <div class="flex items-center gap-1.5"><span class="size-2.5 rounded-full bg-primary"></span> Inbound</div>
            <div class="flex items-center gap-1.5"><span class="size-2.5 rounded-full bg-blue-400"></span> Outbound</div>
          </div>
        </div>
        <svg viewBox="0 0 520 180" class="w-full" style="height:180px">
          <defs>
            <linearGradient id="upGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="var(--primary)" stop-opacity="0.3" />
              <stop offset="100%" stop-color="var(--primary)" stop-opacity="0" />
            </linearGradient>
          </defs>
          <line v-for="i in 4" :key="i" :y1="i * 40" :y2="i * 40" x1="16" x2="504" stroke="currentColor" stroke-opacity="0.08" />
          <path :d="downChart.line" fill="none" stroke="#60a5fa" stroke-width="1.5" stroke-dasharray="4 3" />
          <path :d="upChart.area" fill="url(#upGrad)" />
          <path :d="upChart.line" fill="none" stroke="var(--primary)" stroke-width="2" />
        </svg>
        <div class="mt-1 flex justify-between px-4 text-xs text-muted-foreground">
          <span v-for="t in timeline" :key="t">{{ t }}</span>
        </div>
      </div>

      <div class="border-border bg-card text-card-foreground rounded-xl border p-5">
        <div class="mb-4">
          <h3 class="font-semibold">Node Load</h3>
          <p class="text-muted-foreground text-sm">当前节点资源负载分布</p>
        </div>
        <div class="flex h-40 items-end gap-2">
          <div v-for="node in topNodes" :key="node.name" class="flex flex-1 flex-col items-center gap-1">
            <span class="text-muted-foreground text-[10px] font-medium">{{ node.load }}%</span>
            <div class="bg-primary/80 hover:bg-primary w-full rounded-t transition-colors"
                 :style="{ height: `${node.load}%` }" />
          </div>
        </div>
        <div class="mt-4 border-t border-border pt-4 flex items-center justify-between">
          <div class="flex items-center gap-2 text-primary font-semibold text-sm">
            <Zap class="size-4" /> 加速引擎活动中
          </div>
          <span class="text-xs text-muted-foreground italic">Optimal</span>
        </div>
      </div>
    </div>

    <div class="grid gap-4 lg:grid-cols-3">
      <div class="border-border bg-card text-card-foreground rounded-xl border lg:col-span-2 overflow-hidden">
        <div class="border-b border-border p-5 flex justify-between items-center">
          <h3 class="font-semibold text-sm">High-Traffic Nodes</h3>
          <button class="text-muted-foreground hover:text-foreground"><MoreHorizontal class="size-4"/></button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
            <tr class="border-b border-border bg-muted/30">
              <th class="px-5 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Node</th>
              <th class="px-5 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">IP Address</th>
              <th class="px-5 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Traffic</th>
              <th class="px-5 py-3 text-right text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</th>
            </tr>
            </thead>
            <tbody class="divide-y divide-border">
            <tr v-for="node in topNodes" :key="node.name" class="hover:bg-muted/30 transition-colors">
              <td class="px-5 py-3 font-medium">{{ node.name }}</td>
              <td class="px-5 py-3 text-muted-foreground font-mono text-xs">{{ node.ip }}</td>
              <td class="px-5 py-3 font-semibold">{{ node.traffic }}</td>
              <td class="px-5 py-3 text-right">
                  <span :class="node.status === 'Healthy' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30' : 'bg-amber-100 text-amber-700'"
                        class="px-2.5 py-0.5 rounded-full text-xs font-medium">
                    {{ node.status }}
                  </span>
              </td>
            </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 overflow-hidden flex flex-col">
        <h3 class="font-semibold mb-4">Audit Logs</h3>
        <div class="space-y-4 flex-1">
          <div v-for="(log, i) in auditLogs" :key="i" class="flex items-center gap-3">
            <div :class="[toneMap[log.tone], 'size-2 rounded-full shadow-sm shrink-0']"></div>
            <div class="flex-1 min-w-0">
              <p class="text-xs font-medium truncate">{{ log.action }}</p>
              <p class="text-[10px] text-muted-foreground">{{ log.time }} · {{ log.user }}</p>
            </div>
            <div class="text-[10px] text-muted-foreground italic">{{ log.target }}</div>
          </div>
        </div>
        <button class="mt-4 w-full py-2 border border-border rounded-md text-xs font-medium hover:bg-muted transition-colors">
          View All Logs
        </button>
      </div>
    </div>
  </div>
</template>