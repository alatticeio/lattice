<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useTopologyStore } from '@/stores/topology'
import { storeToRefs } from 'pinia'
import SideDrawer from "@/components/SideDrawer.vue";
import Icon from "@/components/icons/Icon.vue";

const topoStore = useTopologyStore()
const { nodes, links, viewState } = storeToRefs(topoStore)

const isDrawerOpen = ref(false)

// 使用 topoStore.viewState.scale 替换本地的 scale.value
const viewBox = computed(() => {
  const w = 840 / viewState.value.scale
  const h = 520 / viewState.value.scale
  const x = viewState.value.offset.x
  const y = viewState.value.offset.y
  return `${x} ${y} ${w} ${h}`
})

const props = defineProps({
  nodes: { type: Array, required: true },
  links: { type: Array, required: true },
})
// const emit = defineEmits(['select-node'])

const scale = ref(1)
const offset = ref({ x: 0, y: 0 })


const nodeById = computed(() => {
  const m = new Map()
  for (const n of props.nodes) m.set(n.id, n)
  return m
})

function linkClass(l) {
  if (l.quality === 'good') return 'stroke-success'
  if (l.quality === 'warn') return 'stroke-warning'
  return 'stroke-error'
}

function nodeFill(n) {
  if (n.status === 'online') return 'hsl(var(--su))'
  return 'hsl(var(--n))'
}

const selectedNode = ref()

function onSelect(n) {
  selectedNode.value = n
  topoStore.selectNode(n.id)
}

function onZoomEvent(e) {
  const d = e.detail?.delta || 0
  scale.value = Math.min(2.2, Math.max(0.6, +(scale.value + d).toFixed(2)))
}

function onCenterEvent() {
  scale.value = 1
  offset.value = { x: 0, y: 0 }
}

onMounted(() => {
  topoStore.fetchTopology()
  window.addEventListener('wf-topology-zoom', onZoomEvent)
  window.addEventListener('wf-topology-center', onCenterEvent)
})
onBeforeUnmount(() => {
  window.removeEventListener('wf-topology-zoom', onZoomEvent)
  window.removeEventListener('wf-topology-center', onCenterEvent)
})
</script>
<template>
  <div class="w-full h-[520px] bg-base-100 transition-colors duration-300">
    <svg class="w-full h-full select-none" :viewBox="viewBox">
      <defs>
        <filter id="wfShadow" x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow dx="0" dy="2" stdDeviation="2"
                        flood-color="black"
                        flood-opacity="0.2" />
        </filter>
      </defs>

      <g>
        <line
            v-for="l in props.links"
            :key="l.id"
            :x1="nodeById.get(l.from)?.x"
            :y1="nodeById.get(l.from)?.y"
            :x2="nodeById.get(l.to)?.x"
            :y2="nodeById.get(l.to)?.y"
            class="stroke-[3] opacity-50"
            :class="linkClass(l)"
        />
      </g>

      <g>
        <g
            v-for="n in props.nodes"
            :key="n.id"
            class="cursor-pointer group"
            @click="onSelect(n)"
        >
          <circle
              :cx="n.x"
              :cy="n.y"
              r="18"
              filter="url(#wfShadow)"
              :class="n.status === 'online' ? 'fill-success' : 'fill-neutral'"
          />

          <circle
              :cx="n.x"
              :cy="n.y"
              r="26"
              fill="transparent"
              class="stroke-base-content/10"
              stroke-width="2"
          />

          <text
              :x="n.x"
              :y="n.y + 42"
              text-anchor="middle"
              class="fill-base-content font-bold text-[12px] opacity-80"
          >
            {{ n.name }}
          </text>
          <text
              :x="n.x"
              :y="n.y + 58"
              text-anchor="middle"
              class="fill-base-content/50 font-mono text-[11px]"
          >
            {{ n.ip }}
          </text>
        </g>
      </g>
    </svg>

    <SideDrawer v-model="topoStore.isDetailPanelOpen">
      <div class="border-b border-base-content/10 bg-base-200 p-6">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2">
              <span class="badge badge-primary badge-sm font-black italic">NODE INSPECTOR</span>
              <div v-if="selectedNode?.status === 'online'" class="badge badge-success badge-outline badge-xs font-bold">LIVE</div>
            </div>
            <h2 class="mt-2 text-2xl font-black tracking-tighter">{{ selectedNode?.name }}</h2>
            <p class="text-xs font-mono opacity-50">{{ selectedNode?.ip }}</p>
          </div>
<!--          <button @click="topoStore.closeDetailPanel()" class="btn btn-circle btn-sm btn-ghost text-base-content/30">✕</button>-->
        </div>
      </div>

      <div class="p-5 space-y-6 bg-base-100 overflow-y-auto h-[calc(100vh-140px)] custom-scrollbar">

        <div class="grid grid-cols-2 gap-3">
          <div class="stat bg-base-200 rounded-2xl p-3 border border-base-content/5">
            <div class="stat-title text-[10px] font-black uppercase opacity-50">CPU Load</div>
            <div class="stat-value text-lg text-primary">{{ topoStore.nodeMetrics.cpu_trend.at(-1) }}%</div>
            <progress class="progress progress-primary w-full h-1 mt-2" :value="topoStore.nodeMetrics.cpu_trend.at(-1)" max="100"></progress>
          </div>
          <div class="stat bg-base-200 rounded-2xl p-3 border border-base-content/5">
            <div class="stat-title text-[10px] font-black uppercase opacity-50">Memory</div>
            <div class="stat-value text-lg text-secondary">1.4 GB</div>
            <div class="stat-desc text-[9px] opacity-40 italic">of 4.0 GB Total</div>
          </div>
        </div>

        <div class="space-y-3">
          <div class="flex justify-between items-end px-1">
            <h3 class="text-[10px] font-black opacity-50 uppercase tracking-widest">Throughput Trend (Mbps)</h3>
            <div class="flex gap-2 text-[9px] font-bold">
              <span class="text-primary">● TX</span>
              <span class="text-base-content/30">● RX</span>
            </div>
          </div>
          <div class="bg-base-200 border border-base-content/5 rounded-2xl p-4 h-40 relative flex items-end gap-1">
            <div v-for="(val, i) in topoStore.nodeMetrics.net_tx" :key="i"
                 class="flex-1 bg-primary/40 rounded-t-sm transition-all duration-500"
                 :style="{ height: val + '%' }">
            </div>
            <div class="absolute inset-0 flex items-center justify-center opacity-[0.03] pointer-events-none">
              <Icon name="monitor" class="w-24 h-24" />
            </div>
          </div>
        </div>

        <div class="space-y-3">
          <h3 class="text-[10px] font-black opacity-50 uppercase tracking-widest px-1">Active Peers (Tunnels)</h3>
          <div class="space-y-2">
            <div v-for="link in activePeers" :key="link.id"
                 class="flex items-center justify-between p-3 bg-base-200/50 rounded-xl border border-base-content/5 group hover:border-primary/30 transition-all">
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-lg bg-base-300 flex items-center justify-center text-xs font-black">
                  {{ link.to[0].toUpperCase() }}
                </div>
                <div class="flex flex-col">
                  <span class="text-xs font-bold">{{ link.to }}</span>
                  <span class="text-[9px] opacity-40 font-mono italic">{{ link.latency }}ms / {{ link.quality }}</span>
                </div>
              </div>
              <div :class="['w-1.5 h-1.5 rounded-full', link.quality === 'good' ? 'bg-success shadow-[0_0_5px_hsl(var(--su))]' : 'bg-warning']"></div>
            </div>
          </div>
        </div>

        <div class="pt-4 grid grid-cols-1 gap-2">
          <button class="btn btn-primary btn-md font-black text-xs uppercase shadow-lg shadow-primary/20">
            <Icon name="terminal" class="w-4 h-4 mr-2" />
            打开远程控制台 (WebSSH)
          </button>
          <div class="grid grid-cols-2 gap-2">
            <button class="btn btn-outline btn-sm text-[10px] font-black uppercase">抓包分析</button>
            <button class="btn btn-ghost text-error btn-sm text-[10px] font-black uppercase hover:bg-error/10">断开对等关系</button>
          </div>
        </div>

      </div>
    </SideDrawer>
  </div>
</template>