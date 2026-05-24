<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ZoomIn, ZoomOut, Maximize2, X, RefreshCw } from 'lucide-vue-next'
import { listPeer } from '@/api/user'
import request from '@/api/request'

definePage({
  meta: { title: '网络拓扑', description: '可视化网络节点连接拓扑图。' },
})

// ── Types ─────────────────────────────────────────────────────────
type NodeStatus  = 'online' | 'offline' | 'relay'
type NodeType    = 'gateway' | 'peer' | 'relay'
type LinkType    = 'p2p' | 'relay'
type Verdict     = 'FORWARDED' | 'RELAY' | 'DROPPED'

interface TopoNode {
  id: string
  name: string
  type: NodeType
  region: string
  ip: string
  load: number
  txKbps: number
  rxKbps: number
  x: number
  y: number
  status: NodeStatus
}

interface TopoLink {
  id: string
  source: string
  target: string
  quality: number
  latencyMs: number
  type: LinkType
  txKbps: number
  verdict: Verdict
}

// ── State ──────────────────────────────────────────────────────────
const nodes  = ref<TopoNode[]>([])
const links  = ref<TopoLink[]>([])
const loading = ref(true)

// ── Live flow counter ──────────────────────────────────────────────
const flowsPerSec   = ref(0)
const flowTimer     = ref<ReturnType<typeof setInterval> | null>(null)
const refreshTimer  = ref<ReturnType<typeof setInterval> | null>(null)

// ── Verdict counters ───────────────────────────────────────────────
const verdictCounts = computed(() => ({
  FORWARDED: links.value.filter(l => l.verdict === 'FORWARDED').length,
  RELAY:     links.value.filter(l => l.verdict === 'RELAY').length,
  DROPPED:   links.value.filter(l => l.verdict === 'DROPPED').length,
}))

// ── Region groupings ───────────────────────────────────────────────
const regions = computed(() => {
  const map = new Map<string, TopoNode[]>()
  for (const n of nodes.value) {
    if (!map.has(n.region)) map.set(n.region, [])
    map.get(n.region)!.push(n)
  }
  return Array.from(map.entries()).map(([region, ns]) => {
    const xs = ns.map(n => n.x)
    const ys = ns.map(n => n.y)
    const pad = 48
    return {
      region,
      x: Math.min(...xs) - pad,
      y: Math.min(...ys) - pad,
      w: Math.max(...xs) - Math.min(...xs) + pad * 2 + 32,
      h: Math.max(...ys) - Math.min(...ys) + pad * 2 + 32,
    }
  })
})

// ── Stats ──────────────────────────────────────────────────────────
const stats = computed(() => {
  const online  = nodes.value.filter(n => n.status === 'online').length
  const offline = nodes.value.filter(n => n.status === 'offline').length
  const relay   = nodes.value.filter(n => n.status === 'relay').length
  const activeLat = links.value.filter(l => l.latencyMs > 0).map(l => l.latencyMs)
  const avgLatency = activeLat.length ? Math.round(activeLat.reduce((a, b) => a + b, 0) / activeLat.length) : 0
  const health = nodes.value.length ? Math.round(((online + relay) / nodes.value.length) * 100) : 0
  return { total: nodes.value.length, online, offline, relay, avgLatency, health }
})

// ── Layout ─────────────────────────────────────────────────────────
function circularLayout(count: number): Array<{ x: number; y: number }> {
  if (count === 0) return []
  if (count === 1) return [{ x: 440, y: 280 }]
  const cx = 440, cy = 280
  const r = count <= 4 ? 150 : count <= 8 ? 210 : count <= 12 ? 265 : 310
  return Array.from({ length: count }, (_, i) => ({
    x: Math.round(cx + r * Math.cos(2 * Math.PI * i / count - Math.PI / 2)),
    y: Math.round(cy + r * Math.sin(2 * Math.PI * i / count - Math.PI / 2)),
  }))
}

function buildLinks() {
  const arr = nodes.value
  const n   = arr.length
  const out: TopoLink[] = []

  if (n <= 8) {
    // Full mesh
    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        const s = arr[i], t = arr[j]
        const up = s.status === 'online' && t.status === 'online'
        out.push({
          id: `l-${s.id}-${t.id}`,
          source: s.id, target: t.id,
          quality: up ? 85 : 0,
          latencyMs: up ? Math.floor(Math.random() * 75 + 5) : 0,
          type: 'p2p', txKbps: 0,
          verdict: up ? 'FORWARDED' : 'DROPPED',
        })
      }
    }
  } else {
    // Star: relay/gateway as hub, else first online node
    const hub = arr.find(n => n.type !== 'peer')
               ?? arr.find(n => n.status === 'online')
               ?? arr[0]
    if (!hub) return
    for (const peer of arr) {
      if (peer.id === hub.id) continue
      const up = hub.status === 'online' && peer.status === 'online'
      out.push({
        id: `l-${hub.id}-${peer.id}`,
        source: hub.id, target: peer.id,
        quality: up ? 85 : 0,
        latencyMs: up ? Math.floor(Math.random() * 75 + 5) : 0,
        type: 'p2p', txKbps: 0,
        verdict: up ? 'FORWARDED' : 'DROPPED',
      })
    }
  }
  links.value = out
}

async function fetchTopology() {
  loading.value = true
  try {
    const res  = await listPeer({ page: 1, pageSize: 500 })
    const peers: any[] = res?.data?.list ?? []

    const positions = circularLayout(peers.length)
    nodes.value = peers.map((p: any, i: number) => {
      const label  = (p.displayName || p.name || p.appId || '').toLowerCase()
      const type: NodeType =
        label.includes('relay')   ? 'relay'
        : label.includes('gw') || label.includes('gateway') ? 'gateway'
        : 'peer'
      const status: NodeStatus = p.status === 'online' ? 'online' : 'offline'
      const addr = p.address ? String(p.address).split('/')[0] : '—'
      return {
        id:     p.appId || p.name || String(p.id),
        name:   p.displayName || p.name || p.appId || `node-${i}`,
        type,   region: p.namespace || 'default',
        ip:     addr,
        load:   0, txKbps: 0, rxKbps: 0,
        x: positions[i].x,
        y: positions[i].y,
        status,
      }
    })

    // Enrich with CPU metrics from topology endpoint (pro only — silently ignore on failure)
    try {
      const topo = await request.get<{ code: number; data: any[] }>('/monitor/topology')
      for (const snap of (topo?.data ?? [])) {
        const node = nodes.value.find(n =>
          n.id === snap.id || n.name === snap.name || n.id === snap.name)
        if (!node) continue
        node.load = Math.round(parseFloat(snap.metrics?.cpu_usage_percent ?? '0') || 0)
      }
    } catch { /* pro-only */ }

    buildLinks()

    const online = nodes.value.filter(n => n.status === 'online').length
    flowsPerSec.value = online * 120 + Math.floor(Math.random() * 200)
  } catch (e) {
    console.error('Topology fetch failed:', e)
  } finally {
    loading.value = false
  }
}

// ── Canvas state ───────────────────────────────────────────────────
const scale      = ref(1)
const translateX = ref(0)
const translateY = ref(0)
// const svgEl      = ref<SVGSVGElement | null>(null)

const selectedNode = ref<TopoNode | null>(null)
const dragging     = ref<{ nodeId: string; ox: number; oy: number } | null>(null)
const panning      = ref<{ ox: number; oy: number } | null>(null)
const hoveredLink  = ref<string | null>(null)

// ── Helpers ────────────────────────────────────────────────────────
function getNode(id: string) { return nodes.value.find(n => n.id === id) }

function linkPath(link: TopoLink): string {
  const s = getNode(link.source)
  const t = getNode(link.target)
  if (!s || !t) return ''
  const dx = t.x - s.x
  const dy = t.y - s.y
  const cx = (s.x + t.x) / 2 - dy * 0.18
  const cy = (s.y + t.y) / 2 + dx * 0.18
  return `M ${s.x} ${s.y} Q ${cx} ${cy} ${t.x} ${t.y}`
}

function linkMidpoint(link: TopoLink) {
  const s = getNode(link.source)
  const t = getNode(link.target)
  if (!s || !t) return { x: 0, y: 0 }
  const dx = t.x - s.x
  const dy = t.y - s.y
  const cx = (s.x + t.x) / 2 - dy * 0.18
  const cy = (s.y + t.y) / 2 + dx * 0.18
  return {
    x: 0.25 * s.x + 0.5 * cx + 0.25 * t.x,
    y: 0.25 * s.y + 0.5 * cy + 0.25 * t.y,
  }
}

function verdictColor(v: Verdict): string {
  if (v === 'FORWARDED') return '#22d3ee'  // cyan
  if (v === 'RELAY')     return '#a78bfa'  // violet
  return '#f87171'                          // red
}

function verdictBg(v: Verdict): string {
  if (v === 'FORWARDED') return 'rgba(34,211,238,0.15)'
  if (v === 'RELAY')     return 'rgba(167,139,250,0.15)'
  return 'rgba(248,113,113,0.15)'
}

function linkStrokeColor(link: TopoLink): string {
  return verdictColor(link.verdict)
}

function nodeRadius(type: NodeType): number {
  if (type === 'gateway') return 22
  if (type === 'relay')   return 18
  return 15
}

function nodeColor(type: NodeType, status: NodeStatus): string {
  if (status === 'offline') return '#52525b'
  if (type === 'gateway')   return '#22d3ee'
  if (type === 'relay')     return '#a78bfa'
  return '#34d399'
}

function glowId(type: NodeType, status: NodeStatus): string {
  if (status === 'offline') return 'glow-off'
  if (type === 'gateway')   return 'glow-cyan'
  if (type === 'relay')     return 'glow-violet'
  return 'glow-green'
}

function fmtKbps(kbps: number): string {
  if (!kbps) return '—'
  if (kbps >= 1024) return (kbps / 1024).toFixed(1) + ' MB/s'
  return kbps + ' KB/s'
}

function nodeConnCount(id: string): number {
  return links.value.filter(l => (l.source === id || l.target === id) && l.quality > 0).length
}

function particleDuration(link: TopoLink): number {
  // faster for high-traffic links
  const base = 2.5
  return link.verdict === 'DROPPED' ? 0 : Math.max(0.8, base - link.txKbps / 3000)
}

// ── Zoom / Pan / Drag ──────────────────────────────────────────────
function zoom(delta: number) {
  scale.value = Math.max(0.25, Math.min(3, scale.value + delta))
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

// ── Lifecycle ──────────────────────────────────────────────────────
onMounted(() => {
  fetchTopology()
  flowTimer.value = setInterval(() => {
    const delta = Math.floor((Math.random() - 0.5) * 80)
    flowsPerSec.value = Math.max(50, flowsPerSec.value + delta)
  }, 1000)
  refreshTimer.value = setInterval(fetchTopology, 30_000)
})

onUnmounted(() => {
  if (flowTimer.value)   clearInterval(flowTimer.value)
  if (refreshTimer.value) clearInterval(refreshTimer.value)
})
</script>

<template>
  <div class="flex h-full flex-col bg-zinc-950 text-zinc-100 select-none">

    <!-- ── Hubble Header Bar ────────────────────────────────────────── -->
    <div class="flex items-center justify-between border-b border-zinc-800 px-5 py-2.5 shrink-0">
      <div class="flex items-center gap-6">
        <span class="text-sm font-semibold tracking-wide text-zinc-100">Network Flow</span>

        <!-- Verdict counters -->
        <div class="flex items-center gap-4 text-xs font-mono">
          <div class="flex items-center gap-1.5">
            <span class="size-1.5 rounded-full bg-cyan-400 shadow-[0_0_6px_#22d3ee]" />
            <span class="text-zinc-400">FORWARDED</span>
            <span class="font-semibold text-cyan-400">{{ verdictCounts.FORWARDED }}</span>
          </div>
          <div class="flex items-center gap-1.5">
            <span class="size-1.5 rounded-full bg-violet-400 shadow-[0_0_6px_#a78bfa]" />
            <span class="text-zinc-400">RELAY</span>
            <span class="font-semibold text-violet-400">{{ verdictCounts.RELAY }}</span>
          </div>
          <div class="flex items-center gap-1.5">
            <span class="size-1.5 rounded-full bg-red-400 shadow-[0_0_6px_#f87171]" />
            <span class="text-zinc-400">DROPPED</span>
            <span class="font-semibold text-red-400">{{ verdictCounts.DROPPED }}</span>
          </div>
        </div>

        <!-- Live flow rate -->
        <div class="flex items-center gap-1.5 text-xs">
          <span class="relative flex size-2">
            <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-cyan-400 opacity-50" />
            <span class="relative inline-flex size-2 rounded-full bg-cyan-500" />
          </span>
          <span class="text-zinc-400">Flows/s</span>
          <span class="font-mono font-semibold text-cyan-400">{{ flowsPerSec.toLocaleString() }}</span>
        </div>
      </div>

      <!-- Right: manual refresh + stats -->
      <div class="flex items-center gap-5 text-xs text-zinc-400">
        <button
          class="flex items-center gap-1.5 text-zinc-500 hover:text-zinc-200 transition-colors"
          :class="loading ? 'pointer-events-none' : ''"
          @click="fetchTopology"
        >
          <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
          <span>Refresh</span>
        </button>
        <span>Nodes: <b class="text-zinc-100">{{ stats.total }}</b></span>
        <span class="text-emerald-400">Online <b>{{ stats.online }}</b></span>
        <span class="text-violet-400">Relay <b>{{ stats.relay }}</b></span>
        <span class="text-zinc-500">Offline <b>{{ stats.offline }}</b></span>
        <span>Avg Latency: <b class="text-zinc-100">{{ stats.avgLatency }} ms</b></span>
        <span>Health: <b :class="stats.health >= 80 ? 'text-emerald-400' : 'text-amber-400'">{{ stats.health }}%</b></span>
      </div>
    </div>

    <!-- ── Canvas ──────────────────────────────────────────────────── -->
    <div class="relative flex-1 overflow-hidden">

      <!-- Loading overlay -->
      <transition enter-active-class="transition-opacity duration-200" leave-active-class="transition-opacity duration-200" enter-from-class="opacity-0" leave-to-class="opacity-0">
        <div v-if="loading && nodes.length === 0" class="absolute inset-0 z-20 flex items-center justify-center bg-zinc-950/80">
          <div class="flex flex-col items-center gap-3 text-zinc-400">
            <RefreshCw class="size-6 animate-spin" />
            <span class="text-xs">Loading topology…</span>
          </div>
        </div>
      </transition>

      <!-- Empty state -->
      <transition enter-active-class="transition-opacity duration-200" leave-active-class="transition-opacity duration-200" enter-from-class="opacity-0" leave-to-class="opacity-0">
        <div v-if="!loading && nodes.length === 0" class="absolute inset-0 z-20 flex items-center justify-center">
          <div class="text-center text-zinc-500">
            <p class="text-sm">No nodes found in this workspace.</p>
            <p class="mt-1 text-xs">Nodes will appear here once they connect.</p>
          </div>
        </div>
      </transition>

      <!-- Dot grid background -->
      <svg class="absolute inset-0 w-full h-full pointer-events-none" xmlns="http://www.w3.org/2000/svg">
        <defs>
          <pattern id="dot-grid" x="0" y="0" width="28" height="28" patternUnits="userSpaceOnUse">
            <circle cx="1" cy="1" r="0.8" fill="#3f3f46" />
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#dot-grid)" />
      </svg>

      <!-- Main SVG canvas -->
      <svg
        ref="svgEl"
        class="absolute inset-0 w-full h-full cursor-grab active:cursor-grabbing"
        @mousedown="onCanvasMouseDown"
        @mousemove="onMouseMove"
        @mouseup="onMouseUp"
        @mouseleave="onMouseUp"
        @wheel.prevent="onWheel"
      >
        <defs>
          <!-- Glow filters -->
          <filter id="glow-cyan" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="4" result="blur" />
            <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
          </filter>
          <filter id="glow-green" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="3" result="blur" />
            <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
          </filter>
          <filter id="glow-violet" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="3.5" result="blur" />
            <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
          </filter>
          <filter id="glow-off" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="1" result="blur" />
            <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
          </filter>
          <!-- Arrow markers per verdict -->
          <marker id="arrow-fwd" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="#22d3ee" opacity="0.8" />
          </marker>
          <marker id="arrow-relay" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="#a78bfa" opacity="0.8" />
          </marker>
          <marker id="arrow-drop" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="#f87171" opacity="0.5" />
          </marker>
        </defs>

        <g :transform="`translate(${translateX},${translateY}) scale(${scale})`">

          <!-- ── Region cluster borders ────────────────────────────── -->
          <g v-for="r in regions" :key="r.region">
            <rect
              :x="r.x" :y="r.y" :width="r.w" :height="r.h"
              rx="14"
              fill="rgba(255,255,255,0.02)"
              stroke="rgba(255,255,255,0.07)"
              stroke-width="1"
              stroke-dasharray="5 4"
            />
            <text
              :x="r.x + 12" :y="r.y + 16"
              class="font-mono"
              font-size="9"
              fill="rgba(255,255,255,0.25)"
              letter-spacing="0.08em"
            >{{ r.region.toUpperCase() }}</text>
          </g>

          <!-- ── Links (paths, particles, verdict labels) ──────────── -->
          <g v-for="link in links" :key="link.id">
            <!-- Ghost path for particle mpath reference -->
            <path
              :id="`path-${link.id}`"
              :d="linkPath(link)"
              fill="none"
              stroke="none"
            />
            <!-- Visible link stroke -->
            <path
              :d="linkPath(link)"
              fill="none"
              :stroke="linkStrokeColor(link)"
              :stroke-width="hoveredLink === link.id ? 2 : 1.2"
              :stroke-opacity="link.quality === 0 ? 0.2 : 0.5"
              :stroke-dasharray="link.type === 'relay' ? '5 4' : 'none'"
              :marker-end="link.verdict === 'FORWARDED' ? 'url(#arrow-fwd)' : link.verdict === 'RELAY' ? 'url(#arrow-relay)' : 'url(#arrow-drop)'"
              class="cursor-pointer transition-all"
              @mouseenter="hoveredLink = link.id"
              @mouseleave="hoveredLink = null"
            />

            <!-- Flowing particles (skip DROPPED) -->
            <template v-if="link.verdict !== 'DROPPED' && link.quality > 0">
              <circle r="3" :fill="verdictColor(link.verdict)" opacity="0.9">
                <animateMotion
                  :dur="`${particleDuration(link)}s`"
                  repeatCount="indefinite"
                  rotate="auto"
                >
                  <mpath :href="`#path-${link.id}`" />
                </animateMotion>
              </circle>
              <!-- Second offset particle for high traffic -->
              <circle v-if="link.txKbps > 800" r="2.5" :fill="verdictColor(link.verdict)" opacity="0.6">
                <animateMotion
                  :dur="`${particleDuration(link)}s`"
                  :begin="`${particleDuration(link) * 0.5}s`"
                  repeatCount="indefinite"
                  rotate="auto"
                >
                  <mpath :href="`#path-${link.id}`" />
                </animateMotion>
              </circle>
            </template>

            <!-- Verdict badge at midpoint (show on hover or always for DROPPED) -->
            <g
              v-if="hoveredLink === link.id || link.verdict === 'DROPPED'"
              :transform="`translate(${linkMidpoint(link).x}, ${linkMidpoint(link).y})`"
            >
              <rect
                x="-28" y="-8" width="56" height="16" rx="4"
                :fill="verdictBg(link.verdict)"
                :stroke="verdictColor(link.verdict)"
                stroke-width="0.6"
                stroke-opacity="0.6"
              />
              <text
                text-anchor="middle" dominant-baseline="middle"
                font-size="7.5" font-weight="600" letter-spacing="0.06em"
                :fill="verdictColor(link.verdict)"
              >{{ link.verdict }}</text>
            </g>

            <!-- Latency label on hover -->
            <g
              v-if="hoveredLink === link.id && link.latencyMs > 0"
              :transform="`translate(${linkMidpoint(link).x}, ${linkMidpoint(link).y + 14})`"
            >
              <text
                text-anchor="middle" dominant-baseline="middle"
                font-size="7" fill="rgba(255,255,255,0.5)" font-family="monospace"
              >{{ link.latencyMs }}ms · {{ fmtKbps(link.txKbps) }}</text>
            </g>
          </g>

          <!-- ── Nodes ──────────────────────────────────────────────── -->
          <g
            v-for="node in nodes"
            :key="node.id"
            class="topo-node cursor-pointer"
            :transform="`translate(${node.x},${node.y})`"
            @mousedown="onNodeMouseDown($event, node)"
          >
            <!-- Outer glow ring (pulse for online) -->
            <circle
              :r="nodeRadius(node.type) + 6"
              :fill="nodeColor(node.type, node.status)"
              :opacity="node.status === 'offline' ? 0.04 : 0.12"
            >
              <animate
                v-if="node.status !== 'offline'"
                attributeName="opacity"
                values="0.08;0.18;0.08"
                dur="2.8s"
                repeatCount="indefinite"
              />
            </circle>

            <!-- Node circle -->
            <circle
              :r="nodeRadius(node.type)"
              :fill="`${nodeColor(node.type, node.status)}22`"
              :stroke="nodeColor(node.type, node.status)"
              stroke-width="1.5"
              :filter="`url(#${glowId(node.type, node.status)})`"
            />

            <!-- Node type icon text -->
            <text
              text-anchor="middle" dominant-baseline="middle"
              font-size="10" font-weight="700"
              :fill="nodeColor(node.type, node.status)"
            >
              {{ node.type === 'gateway' ? 'GW' : node.type === 'relay' ? 'R' : 'P' }}
            </text>

            <!-- Connection count badge -->
            <g :transform="`translate(${nodeRadius(node.type) - 2},${-nodeRadius(node.type) + 2})`">
              <circle r="7" fill="#18181b" stroke="#3f3f46" stroke-width="0.8" />
              <text
                text-anchor="middle" dominant-baseline="middle"
                font-size="6.5" font-weight="600" fill="#a1a1aa"
              >{{ nodeConnCount(node.id) }}</text>
            </g>

            <!-- Node label -->
            <text
              y="32" text-anchor="middle"
              font-size="9" font-weight="500" fill="#e4e4e7"
              class="pointer-events-none"
            >{{ node.name }}</text>
            <text
              y="43" text-anchor="middle"
              font-size="7.5" fill="#71717a" font-family="monospace"
              class="pointer-events-none"
            >{{ node.ip }}</text>
          </g>

        </g>
      </svg>

      <!-- ── Zoom controls ────────────────────────────────────────── -->
      <div class="absolute bottom-5 right-5 flex flex-col gap-1.5">
        <button
          class="flex size-8 items-center justify-center rounded-lg border border-zinc-700 bg-zinc-900 text-zinc-400 hover:text-zinc-100 hover:border-zinc-500 transition-colors"
          @click="zoom(0.15)"
        ><ZoomIn class="size-3.5" /></button>
        <button
          class="flex size-8 items-center justify-center rounded-lg border border-zinc-700 bg-zinc-900 text-zinc-400 hover:text-zinc-100 hover:border-zinc-500 transition-colors"
          @click="zoom(-0.15)"
        ><ZoomOut class="size-3.5" /></button>
        <button
          class="flex size-8 items-center justify-center rounded-lg border border-zinc-700 bg-zinc-900 text-zinc-400 hover:text-zinc-100 hover:border-zinc-500 transition-colors"
          @click="fitView"
        ><Maximize2 class="size-3.5" /></button>
      </div>

      <!-- ── Node detail panel ───────────────────────────────────── -->
      <transition
        enter-active-class="transition-all duration-200"
        enter-from-class="opacity-0 translate-x-4"
        leave-active-class="transition-all duration-150"
        leave-to-class="opacity-0 translate-x-4"
      >
        <div
          v-if="selectedNode"
          class="absolute top-4 right-4 w-64 rounded-xl border border-zinc-700/60 bg-zinc-900/95 backdrop-blur p-4 shadow-2xl"
        >
          <div class="mb-3 flex items-start justify-between">
            <div>
              <p class="font-semibold text-sm text-zinc-100">{{ selectedNode.name }}</p>
              <p class="text-xs font-mono text-zinc-500 mt-0.5">{{ selectedNode.ip }}</p>
            </div>
            <button class="text-zinc-600 hover:text-zinc-300 transition-colors" @click="selectedNode = null">
              <X class="size-4" />
            </button>
          </div>

          <!-- Status pill -->
          <div class="mb-4 flex items-center gap-2">
            <span
              class="h-1.5 w-1.5 rounded-full"
              :class="{
                'bg-emerald-400 shadow-[0_0_6px_#34d399]': selectedNode.status === 'online',
                'bg-violet-400 shadow-[0_0_6px_#a78bfa]': selectedNode.status === 'relay',
                'bg-zinc-500': selectedNode.status === 'offline',
              }"
            />
            <span class="text-xs font-medium text-zinc-300 uppercase tracking-wide">
              {{ selectedNode.status }}
            </span>
            <span class="ml-auto text-xs text-zinc-500">{{ selectedNode.region }}</span>
          </div>

          <!-- Metrics grid -->
          <div class="grid grid-cols-2 gap-2">
            <div class="rounded-lg bg-zinc-800/60 p-2.5">
              <p class="text-[10px] text-zinc-500 mb-0.5">CPU Load</p>
              <p class="text-sm font-semibold" :class="selectedNode.load >= 80 ? 'text-red-400' : selectedNode.load >= 60 ? 'text-amber-400' : 'text-emerald-400'">
                {{ selectedNode.load }}%
              </p>
            </div>
            <div class="rounded-lg bg-zinc-800/60 p-2.5">
              <p class="text-[10px] text-zinc-500 mb-0.5">Connections</p>
              <p class="text-sm font-semibold text-zinc-100">{{ nodeConnCount(selectedNode.id) }}</p>
            </div>
            <div class="rounded-lg bg-zinc-800/60 p-2.5">
              <p class="text-[10px] text-zinc-500 mb-0.5">TX Rate</p>
              <p class="text-sm font-semibold text-cyan-400">{{ fmtKbps(selectedNode.txKbps) }}</p>
            </div>
            <div class="rounded-lg bg-zinc-800/60 p-2.5">
              <p class="text-[10px] text-zinc-500 mb-0.5">RX Rate</p>
              <p class="text-sm font-semibold text-violet-400">{{ fmtKbps(selectedNode.rxKbps) }}</p>
            </div>
          </div>

          <!-- Type badge -->
          <div class="mt-3 flex items-center justify-between text-xs text-zinc-500">
            <span>Type</span>
            <span
              class="rounded px-2 py-0.5 font-mono text-[10px] font-medium uppercase tracking-wider"
              :class="{
                'bg-cyan-950 text-cyan-400': selectedNode.type === 'gateway',
                'bg-violet-950 text-violet-400': selectedNode.type === 'relay',
                'bg-emerald-950 text-emerald-400': selectedNode.type === 'peer',
              }"
            >{{ selectedNode.type }}</span>
          </div>
        </div>
      </transition>

      <!-- ── Legend ─────────────────────────────────────────────── -->
      <div class="absolute bottom-5 left-5 flex flex-col gap-1.5 text-[10px] text-zinc-500">
        <div class="flex items-center gap-1.5">
          <span class="size-1.5 rounded-full bg-cyan-400" /> Gateway / FORWARDED
        </div>
        <div class="flex items-center gap-1.5">
          <span class="size-1.5 rounded-full bg-violet-400" /> Relay / RELAY
        </div>
        <div class="flex items-center gap-1.5">
          <span class="size-1.5 rounded-full bg-emerald-400" /> Peer
        </div>
        <div class="flex items-center gap-1.5">
          <span class="size-1.5 rounded-full bg-red-400" /> DROPPED
        </div>
      </div>
    </div>
  </div>
</template>
