<script setup lang="ts">
import { computed, ref } from 'vue'
import TopologyGraph from '../components/topology/TopologyGraph.vue'
import NodeDetailsDrawer from '../components/topology/NodeDetailsDrawer.vue'
import Icon from '../components/icons/Icon.vue'

const drawerOpen = ref(false)
const selectedId = ref('')

const nodes = ref([
  { id: 'control', name: 'control-plane', ip: '10.0.0.1', status: 'online', role: 'control', x: 420, y: 140 },
  { id: 'edge-a', name: 'edge-a', ip: '10.0.0.2', status: 'online', role: 'edge', x: 220, y: 300 },
  { id: 'edge-b', name: 'edge-b', ip: '10.0.0.3', status: 'online', role: 'edge', x: 420, y: 340 },
  { id: 'edge-c', name: 'edge-c', ip: '10.0.0.4', status: 'offline', role: 'edge', x: 620, y: 300 },
])

const links = ref([
  { id: 'l1', from: 'control', to: 'edge-a', quality: 'good', rttMs: 18 },
  { id: 'l2', from: 'control', to: 'edge-b', quality: 'good', rttMs: 12 },
  { id: 'l3', from: 'control', to: 'edge-c', quality: 'bad', rttMs: 0 },
  { id: 'l4', from: 'edge-a', to: 'edge-b', quality: 'warn', rttMs: 35 },
])

const selectedNode = computed(() => nodes.value.find(n => n.id === selectedId.value) || null)

function onSelectNode(id) {
  selectedId.value = id
  drawerOpen.value = true
}

function closeDrawer() {
  drawerOpen.value = false
}

// 缩放函数保持不变...
function zoomIn() { window.dispatchEvent(new CustomEvent('wf-topology-zoom', { detail: { delta: +0.1 } })) }
function zoomOut() { window.dispatchEvent(new CustomEvent('wf-topology-zoom', { detail: { delta: -0.1 } })) }
function center() { window.dispatchEvent(new CustomEvent('wf-topology-center')) }
</script>

<template>
  <div class="max-w-7xl mx-auto p-4 lg:p-8 space-y-6">

    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 border-b border-base-300 pb-6">
      <div>
        <h1 class="text-3xl font-extrabold tracking-tight flex items-center gap-3">
          <div class="p-2 bg-info/10 rounded-xl text-info">
            <svg class="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 20l-5.447-2.724A2 2 0 013 15.483V8.517a2 2 0 011.553-1.793L9 4m0 16v-8m0 8l5.447-2.724A2 2 0 0016 14.517V7.517a2 2 0 00-1.553-1.793L9 4m0 12V4m0 0l5.447 2.724A2 2 0 0121 8.517v6.966a2 2 0 01-1.553 1.793L9 20" />
            </svg>
          </div>
          网络拓扑仪表盘
        </h1>
        <p class="text-sm opacity-50 mt-2 max-w-xl">
          实时可视化集群节点间的连接链路。监控 RTT 延迟与链路质量，快速定位网络中断节点。
        </p>
      </div>

      <div class="join bg-base-100 border border-base-300 shadow-sm overflow-hidden rounded-xl">
        <button class="btn btn-ghost btn-sm join-item border-r border-base-300" @click="zoomOut">
          <Icon name="zoom-out" class="w-4 h-4 opacity-70" />
        </button>
        <button class="btn btn-ghost btn-sm join-item border-r border-base-300" @click="zoomIn">
          <Icon name="zoom-in" class="w-4 h-4 opacity-70" />
        </button>
        <button class="btn btn-ghost btn-sm join-item" @click="center">
          <Icon name="center" class="w-4 h-4 opacity-70" />
        </button>
      </div>
    </div>

    <div class="drawer drawer-end overflow-hidden rounded-3xl border border-base-300 shadow-2xl bg-[#0f172a]"> <input id="wf-topology-detail" type="checkbox" class="drawer-toggle" v-model="drawerOpen" />

      <div class="drawer-content relative min-h-[600px]">
        <TopologyGraph
            :nodes="nodes"
            :links="links"
            @select-node="onSelectNode"
        />

        <div class="absolute top-6 right-6 pointer-events-none">
          <div class="bg-slate-900/80 backdrop-blur-md border border-white/10 p-4 rounded-2xl shadow-2xl text-white pointer-events-auto">
            <div class="text-[10px] font-bold uppercase tracking-widest text-base-content/40 mb-3">Link Quality / 链路质量</div>

            <div class="space-y-2">
              <div class="flex items-center gap-3 text-xs">
                <span class="w-8 h-1 bg-success rounded-full shadow-[0_0_8px_rgba(34,197,94,0.6)]"></span>
                <span class="font-medium">良好</span>
                <span class="opacity-40 font-mono text-[10px] ml-auto">&lt; 30ms</span>
              </div>
              <div class="flex items-center gap-3 text-xs">
                <span class="w-8 h-1 bg-warning rounded-full shadow-[0_0_8px_rgba(234,179,8,0.6)]"></span>
                <span class="font-medium">一般</span>
                <span class="opacity-40 font-mono text-[10px] ml-auto">30-80ms</span>
              </div>
              <div class="flex items-center gap-3 text-xs">
                <span class="w-8 h-1 bg-error rounded-full shadow-[0_0_8px_rgba(239,68,68,0.6)]"></span>
                <span class="font-medium">异常</span>
                <span class="opacity-40 font-mono text-[10px] ml-auto">Timeout</span>
              </div>
            </div>

            <div class="mt-4 pt-4 border-t border-white/5 flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-success animate-pulse"></div>
              <span class="text-[10px] font-bold opacity-60 uppercase">Real-time Data Stream</span>
            </div>
          </div>
        </div>

        <div class="absolute bottom-6 left-6 text-white/30 text-[10px] font-mono tracking-widest uppercase italic">
          WireFlow v2.0 Topology Engine // Auto-layout Active
        </div>
      </div>

      <div class="drawer-side z-50">
        <label class="drawer-overlay" for="wf-topology-detail" @click="closeDrawer"></label>
        <div class="w-[450px] max-w-[90vw] min-h-full bg-base-100 border-l border-base-300 shadow-2xl">
          <NodeDetailsDrawer :node="selectedNode" @close="closeDrawer" />
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <div class="stats shadow-sm border border-base-300 bg-base-100">
        <div class="stat">
          <div class="stat-title text-[10px] font-bold uppercase">在线节点</div>
          <div class="stat-value text-2xl font-mono text-success">
            {{ nodes.filter(n=>n.status==='online').length }}
            <span class="text-xs opacity-30 text-base-content">/ {{ nodes.length }}</span>
          </div>
          <div class="stat-desc mt-1">Nodes responding to ping</div>
        </div>
      </div>

      <div class="stats shadow-sm border border-base-300 bg-base-100">
        <div class="stat">
          <div class="stat-title text-[10px] font-bold uppercase">活动链路</div>
          <div class="stat-value text-2xl font-mono">{{ links.length }}</div>
          <div class="stat-desc font-mono uppercase text-[9px] text-info">Mesh Connectivity: 85%</div>
        </div>
      </div>

      <div class="stats shadow-sm border border-base-300 bg-base-100">
        <div class="stat">
          <div class="stat-title text-[10px] font-bold uppercase">交互指引</div>
          <div class="stat-value text-xl">节点详情</div>
          <div class="stat-desc">点击图形中任意节点开启控制面板</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 针对深色拓扑背景的细微调整 */
.drawer-content {
  background-image: radial-gradient(circle at 2px 2px, rgba(255,255,255,0.05) 1px, transparent 0);
  background-size: 32px 32px; /* 给背景增加微弱的点阵网格感 */
}
</style>