<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  ZoomIn, ZoomOut, Maximize2, X, Wifi,
  Globe, Cpu, ArrowUp, ArrowDown, Radio, Server,
} from 'lucide-vue-next'

definePage({
  meta: { title: '网络拓扑', description: '可视化网络节点连接拓扑图。' },
})

// ── Types ─────────────────────────────────────────────────────────
type NodeStatus = 'online' | 'offline' | 'relay'
type NodeType   = 'gateway' | 'peer' | 'relay'
type LinkType   = 'p2p' | 'relay'

interface TopoNode {
  id: string
  name: string
  type: NodeType
  region: string
  ip: string
  load: number      // 0-100 cpu/load
  txKbps: number
  rxKbps: number
  x: number
  y: number
  status: NodeStatus
}

interface TopoLink {
  source: string
  target: string
  quality: number   // 0-100
  latencyMs: number
  type: LinkType
  txKbps: number
}

// ── Mock data ──────────────────────────────────────────────────────
const nodes = ref<TopoNode[]>([
  { id: 'gw',      name: 'gw-hk-01',      type: 'gateway', region: 'ap-east-1',      ip: '10.0.0.1',  load: 34, txKbps: 2840, rxKbps: 1920, x: 450, y: 260, status: 'online'  },
  { id: 'relay1',  name: 'relay-sg-01',   type: 'relay',   region: 'ap-southeast-1', ip: '10.0.0.10', load: 71, txKbps: 4210, rxKbps: 3870, x: 450, y: 430, status: 'relay'   },
  { id: 'alpha',   name: 'node-alpha',    type: 'peer',    region: 'us-west-2',      ip: '10.0.1.10', load: 52, txKbps: 1240, rxKbps:  880, x: 680, y: 150, status: 'online'  },
  { id: 'beta',    name: 'node-beta',     type: 'peer',    region: 'eu-central-1',   ip: '10.0.1.11', load: 18, txKbps:  430, rxKbps:  620, x: 750, y: 340, status: 'online'  },
  { id: 'gamma',   name: 'node-gamma',    type: 'peer',    region: 'us-east-1',      ip: '10.0.1.12', load:  0, txKbps:    0, rxKbps:    0, x: 200, y: 160, status: 'offline' },
  { id: 'delta',   name: 'node-delta',    type: 'peer',    region: 'ap-southeast-1', ip: '10.0.1.13', load: 41, txKbps:  760, rxKbps:  540, x: 650, y: 490, status: 'online'  },
  { id: 'epsilon', name: 'node-epsilon',  type: 'peer',    region: 'eu-west-1',      ip: '10.0.1.14', load: 63, txKbps: 1560, rxKbps: 1090, x: 230, y: 430, status: 'online'  },
  { id: 'zeta',    name: 'node-zeta',     type: 'peer',    region: 'ap-northeast-1', ip: '10.0.1.15', load: 29, txKbps:  320, rxKbps:  410, x: 210, y: 310, status: 'online'  },
])

const links = ref<TopoLink[]>([
  { source: 'gw',     target: 'alpha',   quality: 96, latencyMs:  12, type: 'p2p',   txKbps: 1240 },
  { source: 'gw',     target: 'beta',    quality: 88, latencyMs:  28, type: 'p2p',   txKbps:  430 },
  { source: 'gw',     target: 'relay1',  quality: 99, latencyMs:   6, type: 'p2p',   txKbps: 4210 },
  { source: 'gw',     target: 'gamma',   quality:  0, latencyMs:   0, type: 'relay', txKbps:    0 },
  { source: 'gw',     target: 'zeta',    quality: 74, latencyMs:  41, type: 'p2p',   txKbps:  320 },
  { source: 'alpha',  target: 'beta',    quality: 92, latencyMs:  18, type: 'p2p',   txKbps:  810 },
  { source: 'beta',   target: 'delta',   quality: 77, latencyMs:  35, type: 'p2p',   txKbps:  560 },
  { source: 'relay1', target: 'epsilon', quality: 63, latencyMs:  62, type: 'relay', txKbps:  980 },
  { source: 'relay1', target: 'delta',   quality: 55, latencyMs:  78, type: 'relay', txKbps:  640 },
  { source: 'zeta',   target: 'epsilon', quality: 81, latencyMs:  24, type: 'p2p',   txKbps:  420 },
  { source: 'gamma',  target: 'epsilon', quality:  0, latencyMs:   0, type: 'relay', txKbps:    0 },
])

// ── Stats ──────────────────────────────────────────────────────────
const stats = computed(() => {
  const online  = nodes.value.filter(n => n.status === 'online').length
  const offline = nodes.value.filter(n => n.status === 'offline').length
  const relay   = nodes.value.filter(n => n.status === 'relay').length
  const p2p     = links.value.filter(l => l.type === 'p2p' && l.quality > 0).length
  const relayLinks = links.value.filter(l => l.type === 'relay').length
  const activeLat  = links.value.filter(l => l.latencyMs > 0).map(l => l.latencyMs)
  const avgLatency = activeLat.length ? Math.round(activeLat.reduce((a, b) => a + b, 0) / activeLat.length) : 0
  const health  = nodes.value.length ? Math.round(((online + relay) / nodes.value.length) * 100) : 0
  return { total: nodes.value.length, online, offline, relay, p2pLinks: p2p, relayLinks, avgLatency, health }
})

// ── Canvas state ───────────────────────────────────────────────────
const scale      = ref(1)
const translateX = ref(0)
const translateY = ref(0)
const svgEl      = ref<SVGSVGElement | null>(null)

const selectedNode  = ref<TopoNode | null>(null)
const dragging      = ref<{ nodeId: string; ox: number; oy: number } | null>(null)
const panning       = ref<{ ox: number; oy: number } | null>(null)
const hoveredNode   = ref<string | null>(null)

// ── Helpers ────────────────────────────────────────────────────────
function getNode(id: string) { return nodes.value.find(n => n.id === id) }

function nodeLinks(id: string) {
  return links.value.filter(l => l.source === id || l.target === id)
}

function linkPath(link: TopoLink) {
  const s = getNode(link.source)
  const t = getNode(link.target)
  if (!s || !t) return ''
  const dx = t.x - s.x
  const dy = t.y - s.y
  const cx = (s.x + t.x) / 2 - dy * 0.15
  const cy = (s.y + t.y) / 2 + dx * 0.15
  return `M ${s.x} ${s.y} Q ${cx} ${cy} ${t.x} ${t.y}`
}

function linkMidpoint(link: TopoLink) {
  const s = getNode(link.source)
  const t = getNode(link.target)
  if (!s || !t) return { x: 0, y: 0 }
  const dx = t.x - s.x
  const dy = t.y - s.y
  const cx = (s.x + t.x) / 2 - dy * 0.15
  const cy = (s.y + t.y) / 2 + dx * 0.15
  // quadratic bezier midpoint at t=0.5
  return {
    x: 0.25 * s.x + 0.5 * cx + 0.25 * t.x,
    y: 0.25 * s.y + 0.5 * cy + 0.25 * t.y,
  }
}

function qualityColor(q: number): string {
  if (q === 0)  return '#71717a'
  if (q >= 85)  return 'oklch(0.6 0.18 145)'   // green
  if (q >= 60)  return 'oklch(0.7 0.18 80)'    // amber
  return            'oklch(0.6 0.22 25)'         // red
}

function nodeRadius(type: NodeType): number {
  if (type === 'gateway') return 24
  if (type === 'relay')   return 20
  return 16
}

function nodeGlowColor(status: NodeStatus): string {
  if (status === 'online') return 'oklch(0.6 0.18 145)'
  if (status === 'relay')  return 'var(--color-primary)'
  return '#71717a'
}

function loadColor(load: number): string {
  if (load >= 80) return 'oklch(0.6 0.22 25)'
  if (load >= 60) return 'oklch(0.7 0.18 80)'
  return 'oklch(0.6 0.18 145)'
}

function fmtKbps(kbps: number): string {
  if (!kbps) return '—'
  if (kbps >= 1024) return (kbps / 1024).toFixed(1) + ' MB/s'
  return kbps + ' KB/s'
}

const regionFlag: Record<string, string> = {
  'us-west-2': '🇺🇸', 'us-east-1': '🇺🇸',
  'eu-central-1': '🇩🇪', 'eu-west-1': '🇬🇧',
  'ap-east-1': '🇭🇰', 'ap-southeast-1': '🇸🇬', 'ap-northeast-1': '🇯🇵',
}

// ── Zoom / Pan / Drag ──────────────────────────────────────────────
function zoom(delta: number) {
  scale.value = Math.max(0.3, Math.min(3, scale.value + delta))
}

function fitView() {
  scale.value = 1
  translateX.value = 0
  translateY.value = 0
}

function onNodeMouseDown(e: MouseEvent, node: TopoNode) {
  e.stopPropagation()
  selectedNode.value = node
  dragging.value = {
    nodeId: node.id,
    ox: (e.clientX - translateX.value) / scale.value - node.x,
    oy: (e.clientY - translateY.value) / scale.value - node.y,
  }
}

function onCanvasMouseDown(e: MouseEvent) {
  if ((e.target as Element).closest('.topo-node')) return
  panning.value = { ox: e.clientX - translateX.value, oy: e.clientY - translateY.value }
}

function onMouseMove(e: MouseEvent) {
  if (dragging.value) {
    const node = nodes.value.find(n => n.id === dragging.value!.nodeId)
    if (node) {
      node.x = (e.clientX - translateX.value) / scale.value - dragging.value.ox
      node.y = (e.clientY - translateY.value) / scale.value - dragging.value.oy
    }
  } else if (panning.value) {
    translateX.value = e.clientX - panning.value.ox
    translateY.value = e.clientY - panning.value.oy
  }
}

function onMouseUp() {
  dragging.value = null
  panning.value  = null
}

function onWheel(e: WheelEvent) {
  e.preventDefault()
  zoom(e.deltaY < 0 ? 0.1 : -0.1)
}

onMounted(() => {
  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
  svgEl.value?.setAttribute('data-ready', 'true')
})
onUnmounted(() => {
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', onMouseUp)
})
</script>

<style scoped>
.link-flow {
  animation: dash-flow 1.2s linear infinite;
}
.link-flow-slow {
  animation: dash-flow 2.5s linear infinite;
}
@keyframes dash-flow {
  to { stroke-dashoffset: -24; }
}
.node-pulse {
  animation: node-ring 2s ease-out infinite;
}
@keyframes node-ring {
  0%   { r: 0px; opacity: 0.6; }
  100% { r: 36px; opacity: 0; }
}
</style>

<template>
  <div class="flex flex-col h-full overflow-hidden">

    <!-- ── Stats bar ──────────────────────────────────────────────── -->
    <div class="flex items-center gap-2 px-4 py-2 border-b border-border bg-card shrink-0 flex-wrap">
      <!-- Health indicator -->
      <div class="flex items-center gap-2 pr-3 border-r border-border">
        <div class="relative size-7">
          <svg viewBox="0 0 28 28" class="size-7 -rotate-90">
            <circle cx="14" cy="14" r="11" fill="none" stroke="var(--muted)" stroke-width="3" />
            <circle cx="14" cy="14" r="11" fill="none"
              :stroke="stats.health >= 80 ? 'oklch(0.6 0.18 145)' : 'oklch(0.7 0.18 80)'"
              stroke-width="3" stroke-linecap="round"
              :stroke-dasharray="`${stats.health * 0.691} 69.1`"
            />
          </svg>
          <span class="absolute inset-0 flex items-center justify-center text-[8px] font-black rotate-90">
            {{ stats.health }}
          </span>
        </div>
        <div>
          <p class="text-[10px] text-muted-foreground leading-none">网络健康</p>
          <p class="text-xs font-bold leading-none mt-0.5">{{ stats.health }}%</p>
        </div>
      </div>

      <!-- Node stats -->
      <div class="flex items-center gap-2">
        <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
          <Server class="size-3 shrink-0" />
          <span class="font-semibold text-foreground">{{ stats.total }}</span> 节点
        </span>
        <span class="flex items-center gap-1.5 text-xs">
          <span class="size-2 rounded-full bg-emerald-500 shrink-0" />
          <span class="font-semibold text-emerald-500">{{ stats.online }}</span>
          <span class="text-muted-foreground">在线</span>
        </span>
        <span class="flex items-center gap-1.5 text-xs">
          <span class="size-2 rounded-full bg-zinc-400 shrink-0" />
          <span class="font-semibold text-muted-foreground">{{ stats.offline }}</span>
          <span class="text-muted-foreground">离线</span>
        </span>
        <span class="flex items-center gap-1.5 text-xs">
          <Radio class="size-3 text-primary shrink-0" />
          <span class="font-semibold text-primary">{{ stats.relay }}</span>
          <span class="text-muted-foreground">中继</span>
        </span>
      </div>

      <div class="w-px h-4 bg-border" />

      <!-- Link stats -->
      <div class="flex items-center gap-2">
        <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
          <span class="h-0.5 w-4 bg-emerald-500 rounded inline-block" /> P2P
          <span class="font-semibold text-foreground">{{ stats.p2pLinks }}</span>
        </span>
        <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
          <span class="h-0.5 w-4 bg-violet-400 rounded inline-block border-dashed" /> 中继
          <span class="font-semibold text-foreground">{{ stats.relayLinks }}</span>
        </span>
      </div>

      <div class="w-px h-4 bg-border" />

      <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Wifi class="size-3 shrink-0" /> 均延迟
        <span class="font-bold text-foreground">{{ stats.avgLatency }} ms</span>
      </span>

      <!-- Controls -->
      <div class="ml-auto flex items-center gap-1">
        <button
          class="size-7 flex items-center justify-center rounded-md border border-border hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
          @click="zoom(-0.15)"
        ><ZoomOut class="size-3.5" /></button>
        <span class="text-xs text-muted-foreground w-11 text-center tabular-nums">{{ Math.round(scale * 100) }}%</span>
        <button
          class="size-7 flex items-center justify-center rounded-md border border-border hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
          @click="zoom(0.15)"
        ><ZoomIn class="size-3.5" /></button>
        <button
          class="size-7 flex items-center justify-center rounded-md border border-border hover:bg-muted transition-colors text-muted-foreground hover:text-foreground ml-0.5"
          @click="fitView"
        ><Maximize2 class="size-3.5" /></button>
      </div>
    </div>

    <!-- ── Main canvas area ───────────────────────────────────────── -->
    <div class="relative flex-1 overflow-hidden bg-muted/10">

      <!-- SVG Canvas -->
      <svg
        ref="svgEl"
        class="w-full h-full select-none"
        :style="{ cursor: dragging ? 'grabbing' : panning ? 'grabbing' : 'grab' }"
        @mousedown="onCanvasMouseDown"
        @wheel.prevent="onWheel"
      >
        <defs>
          <!-- Dot grid pattern -->
          <pattern id="dot-grid" width="24" height="24" patternUnits="userSpaceOnUse">
            <circle cx="1" cy="1" r="0.8" fill="var(--muted-foreground)" fill-opacity="0.18" />
          </pattern>
          <!-- Glow filters -->
          <filter id="glow-green" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="3" result="blur" />
            <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
          </filter>
          <filter id="glow-primary" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="4" result="blur" />
            <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
          </filter>
        </defs>

        <!-- Background -->
        <rect width="100%" height="100%" fill="url(#dot-grid)" />

        <g :transform="`translate(${translateX},${translateY}) scale(${scale})`">

          <!-- ── Links ─────────────────────────────────────────────── -->
          <g v-for="link in links" :key="`${link.source}-${link.target}`">
            <!-- Shadow / base line -->
            <path
              :d="linkPath(link)"
              fill="none"
              :stroke="qualityColor(link.quality)"
              stroke-width="3"
              stroke-opacity="0.08"
            />
            <!-- Active animated line -->
            <path
              :d="linkPath(link)"
              fill="none"
              :stroke="qualityColor(link.quality)"
              :stroke-width="link.quality >= 85 ? 2.5 : 1.8"
              :stroke-dasharray="link.type === 'relay' || link.quality === 0 ? '8 6' : '14 4'"
              :stroke-opacity="link.quality === 0 ? 0.3 : 0.75"
              :class="link.quality > 0 ? (link.type === 'p2p' ? 'link-flow' : 'link-flow-slow') : ''"
            />
            <!-- Latency label at midpoint -->
            <g v-if="link.latencyMs > 0" :transform="`translate(${linkMidpoint(link).x}, ${linkMidpoint(link).y})`">
              <rect x="-16" y="-9" width="32" height="14" rx="4"
                fill="var(--card)" fill-opacity="0.85"
                stroke="var(--border)" stroke-width="0.8"
              />
              <text text-anchor="middle" y="2" font-size="8.5" font-weight="600"
                :style="{ fill: qualityColor(link.quality) }">
                {{ link.latencyMs }}ms
              </text>
            </g>
          </g>

          <!-- ── Nodes ──────────────────────────────────────────────── -->
          <g
            v-for="node in nodes"
            :key="node.id"
            class="topo-node"
            :transform="`translate(${node.x},${node.y})`"
            style="cursor: grab"
            @mousedown="onNodeMouseDown($event, node)"
            @mouseenter="hoveredNode = node.id"
            @mouseleave="hoveredNode = null"
          >
            <!-- Pulse ring (online only) -->
            <circle
              v-if="node.status === 'online'"
              r="0"
              :fill="nodeGlowColor(node.status)"
              fill-opacity="0"
              class="node-pulse"
            />
            <!-- Outer glow area -->
            <circle
              v-if="node.status !== 'offline'"
              :r="nodeRadius(node.type) + 10"
              :fill="nodeGlowColor(node.status)"
              fill-opacity="0.07"
            />
            <!-- Hovered highlight -->
            <circle
              v-if="hoveredNode === node.id || selectedNode?.id === node.id"
              :r="nodeRadius(node.type) + 6"
              fill="none"
              stroke="var(--primary)"
              stroke-width="1.5"
              stroke-opacity="0.5"
              stroke-dasharray="4 2"
            />
            <!-- Main circle -->
            <circle
              :r="nodeRadius(node.type)"
              :fill="nodeGlowColor(node.status)"
              fill-opacity="0.18"
              :stroke="nodeGlowColor(node.status)"
              stroke-width="2"
              :filter="node.status !== 'offline' ? (node.type === 'gateway' ? 'url(#glow-primary)' : 'url(#glow-green)') : ''"
            />
            <!-- Inner filled circle -->
            <circle :r="nodeRadius(node.type) - 6" :fill="nodeGlowColor(node.status)" fill-opacity="0.25" />

            <!-- Type letter -->
            <text
              text-anchor="middle" y="4"
              :font-size="node.type === 'gateway' ? 11 : 9"
              font-weight="800"
              :style="{ fill: nodeGlowColor(node.status) }"
            >
              {{ node.type === 'gateway' ? 'GW' : node.type === 'relay' ? 'R' : 'P' }}
            </text>

            <!-- Status dot (top-right) -->
            <circle
              :cx="nodeRadius(node.type) - 2"
              :cy="-(nodeRadius(node.type) - 2)"
              r="5"
              :fill="nodeGlowColor(node.status)"
              stroke="var(--card)"
              stroke-width="1.5"
            />

            <!-- Load bar (bottom arc) — only for online/relay -->
            <g v-if="node.status !== 'offline'">
              <path
                :d="`M -${nodeRadius(node.type) - 4} ${nodeRadius(node.type) + 8} L ${nodeRadius(node.type) - 4} ${nodeRadius(node.type) + 8}`"
                fill="none"
                stroke="var(--muted)"
                stroke-width="2.5"
                stroke-linecap="round"
              />
              <path
                :d="`M -${nodeRadius(node.type) - 4} ${nodeRadius(node.type) + 8} L ${-((nodeRadius(node.type) - 4)) + (node.load / 100) * (nodeRadius(node.type) - 4) * 2} ${nodeRadius(node.type) + 8}`"
                fill="none"
                :stroke="loadColor(node.load)"
                stroke-width="2.5"
                stroke-linecap="round"
              />
            </g>

            <!-- Name label -->
            <text
              :y="nodeRadius(node.type) + 22"
              text-anchor="middle"
              font-size="10"
              font-weight="600"
              style="fill: var(--foreground)"
            >{{ node.name }}</text>
            <!-- IP label -->
            <text
              :y="nodeRadius(node.type) + 34"
              text-anchor="middle"
              font-size="8.5"
              style="fill: var(--muted-foreground)"
            >{{ node.ip }}</text>
          </g>

        </g>
      </svg>

      <!-- ── Left panel: node list ───────────────────────────────── -->
      <div class="absolute left-3 top-3 w-52 bg-card/90 backdrop-blur border border-border rounded-xl shadow-sm overflow-hidden">
        <div class="px-3 py-2.5 border-b border-border flex items-center justify-between">
          <p class="text-xs font-bold">节点列表</p>
          <span class="text-[10px] text-muted-foreground">{{ nodes.length }} 个</span>
        </div>
        <div class="max-h-80 overflow-y-auto">
          <button
            v-for="node in nodes"
            :key="node.id"
            class="w-full flex items-center gap-2.5 px-3 py-2 hover:bg-muted/50 transition-colors text-left border-b border-border/50 last:border-0"
            :class="selectedNode?.id === node.id ? 'bg-primary/5' : ''"
            @click="selectedNode = selectedNode?.id === node.id ? null : node"
          >
            <!-- Status dot -->
            <span class="relative flex size-2 shrink-0">
              <span
                v-if="node.status !== 'offline'"
                class="absolute inline-flex h-full w-full animate-ping rounded-full opacity-50"
                :class="node.status === 'relay' ? 'bg-primary' : 'bg-emerald-500'"
              />
              <span
                class="relative inline-flex size-2 rounded-full"
                :class="node.status === 'online' ? 'bg-emerald-500' : node.status === 'relay' ? 'bg-primary' : 'bg-zinc-400'"
              />
            </span>

            <div class="min-w-0 flex-1">
              <p class="text-[11px] font-semibold truncate leading-none">{{ node.name }}</p>
              <p class="text-[10px] text-muted-foreground/60 mt-0.5 font-mono">{{ node.ip }}</p>
            </div>

            <span class="text-[9px] text-muted-foreground/60 shrink-0">{{ regionFlag[node.region] ?? '🌐' }}</span>
          </button>
        </div>

        <!-- Legend -->
        <div class="px-3 py-2 border-t border-border bg-muted/20 space-y-1">
          <div class="flex items-center gap-3 text-[10px] text-muted-foreground">
            <span class="flex items-center gap-1"><span class="h-0.5 w-4 bg-emerald-500 rounded inline-block" />P2P 直连</span>
            <span class="flex items-center gap-1"><span class="h-0.5 w-4 bg-violet-400 rounded inline-block" />中继</span>
          </div>
          <div class="flex items-center gap-3 text-[10px] text-muted-foreground">
            <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-emerald-500 inline-block" />在线</span>
            <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-primary inline-block" />中继节点</span>
            <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-zinc-400 inline-block" />离线</span>
          </div>
        </div>
      </div>

      <!-- ── Right panel: node detail ────────────────────────────── -->
      <Transition
        enter-from-class="opacity-0 translate-x-4"
        enter-active-class="transition-all duration-200"
        leave-to-class="opacity-0 translate-x-4"
        leave-active-class="transition-all duration-150"
      >
        <div
          v-if="selectedNode"
          class="absolute right-3 top-3 w-68 bg-card/95 backdrop-blur border border-border rounded-xl shadow-lg overflow-hidden"
          style="width: 264px"
        >
          <!-- Header -->
          <div class="flex items-start justify-between p-3 border-b border-border">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span
                  class="text-[10px] font-bold px-1.5 py-0.5 rounded uppercase tracking-wider"
                  :class="selectedNode.type === 'gateway'
                    ? 'bg-primary/10 text-primary'
                    : selectedNode.type === 'relay'
                      ? 'bg-violet-500/10 text-violet-500'
                      : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'"
                >{{ selectedNode.type }}</span>
                <span
                  class="text-[10px] font-semibold px-1.5 py-0.5 rounded-full"
                  :class="selectedNode.status === 'online'
                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20'
                    : selectedNode.status === 'relay'
                      ? 'bg-primary/10 text-primary ring-1 ring-primary/20'
                      : 'bg-muted text-muted-foreground ring-1 ring-border'"
                >
                  {{ selectedNode.status === 'online' ? '在线' : selectedNode.status === 'relay' ? '中继' : '离线' }}
                </span>
              </div>
              <p class="font-bold text-sm mt-1 truncate">{{ selectedNode.name }}</p>
            </div>
            <button class="text-muted-foreground hover:text-foreground transition-colors shrink-0 ml-2" @click="selectedNode = null">
              <X class="size-4" />
            </button>
          </div>

          <!-- Identity -->
          <div class="px-3 py-2.5 space-y-1.5 border-b border-border">
            <div class="flex items-center justify-between text-xs">
              <span class="text-muted-foreground flex items-center gap-1"><Globe class="size-3" /> 地域</span>
              <span class="font-mono font-medium">{{ regionFlag[selectedNode.region] }} {{ selectedNode.region }}</span>
            </div>
            <div class="flex items-center justify-between text-xs">
              <span class="text-muted-foreground">IP 地址</span>
              <span class="font-mono font-semibold text-foreground">{{ selectedNode.ip }}</span>
            </div>
            <div class="flex items-center justify-between text-xs">
              <span class="text-muted-foreground">连接数</span>
              <span class="font-semibold">{{ nodeLinks(selectedNode.id).length }}</span>
            </div>
          </div>

          <!-- Load -->
          <div class="px-3 py-2.5 border-b border-border">
            <div class="flex items-center justify-between mb-1.5">
              <span class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/60 flex items-center gap-1">
                <Cpu class="size-3" /> 节点负载
              </span>
              <span class="text-xs font-bold tabular-nums" :style="{ color: loadColor(selectedNode.load) }">
                {{ selectedNode.status === 'offline' ? '—' : selectedNode.load + '%' }}
              </span>
            </div>
            <div class="h-1.5 bg-muted rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :style="{ width: `${selectedNode.status === 'offline' ? 0 : selectedNode.load}%`, background: loadColor(selectedNode.load) }"
              />
            </div>
          </div>

          <!-- Traffic -->
          <div class="px-3 py-2.5 border-b border-border grid grid-cols-2 gap-2">
            <div>
              <p class="text-[10px] text-muted-foreground/60 uppercase tracking-wider mb-1 flex items-center gap-1">
                <ArrowUp class="size-3 text-violet-400" /> 发送
              </p>
              <p class="text-xs font-bold tabular-nums">{{ fmtKbps(selectedNode.txKbps) }}</p>
            </div>
            <div>
              <p class="text-[10px] text-muted-foreground/60 uppercase tracking-wider mb-1 flex items-center gap-1">
                <ArrowDown class="size-3 text-blue-400" /> 接收
              </p>
              <p class="text-xs font-bold tabular-nums">{{ fmtKbps(selectedNode.rxKbps) }}</p>
            </div>
          </div>

          <!-- Connected peers -->
          <div class="px-3 pt-2.5 pb-3">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/60 mb-2">
              相连节点 ({{ nodeLinks(selectedNode.id).length }})
            </p>
            <div class="space-y-2 max-h-40 overflow-y-auto">
              <div
                v-for="link in nodeLinks(selectedNode.id)"
                :key="`${link.source}-${link.target}`"
                class="space-y-1"
              >
                <div class="flex items-center justify-between text-[11px]">
                  <span class="font-medium text-muted-foreground truncate">
                    {{ link.source === selectedNode.id ? link.target : link.source }}
                  </span>
                  <div class="flex items-center gap-1.5 shrink-0">
                    <span v-if="link.latencyMs" class="text-muted-foreground/60 font-mono text-[10px]">{{ link.latencyMs }}ms</span>
                    <span
                      class="text-[10px] font-bold"
                      :style="{ color: qualityColor(link.quality) }"
                    >{{ link.quality > 0 ? link.quality + '%' : '离线' }}</span>
                  </div>
                </div>
                <div class="h-1 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all"
                    :style="{ width: `${link.quality}%`, background: qualityColor(link.quality) }"
                  />
                </div>
              </div>
              <p v-if="!nodeLinks(selectedNode.id).length" class="text-[11px] text-muted-foreground/40 italic">无连接</p>
            </div>
          </div>
        </div>
      </Transition>

      <!-- ── Canvas hint ─────────────────────────────────────────── -->
      <div class="absolute bottom-3 left-1/2 -translate-x-1/2 flex items-center gap-1.5 text-[10px] text-muted-foreground/50 bg-card/60 backdrop-blur px-3 py-1.5 rounded-full border border-border/50">
        <span>拖拽节点移动</span>
        <span class="w-px h-3 bg-border" />
        <span>空白处平移画布</span>
        <span class="w-px h-3 bg-border" />
        <span>滚轮缩放</span>
      </div>

    </div>
  </div>
</template>
