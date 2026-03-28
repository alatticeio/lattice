import { defineStore } from 'pinia'

interface TopologyNode {
    id: string | number;
    name: string;
    ip: string;
    x: number;
    y: number;
    status: 'online' | 'offline';
    type: string; // e.g., 'edge', 'relay', 'client'
}

interface TopologyLink {
    id: string | number;
    from: string | number;
    to: string | number;
    quality: 'good' | 'warn' | 'error';
    latency: number;
    bandwidth: string;
}

export const useTopologyStore = defineStore('topology', {
    state: () => ({
        nodes: [] as TopologyNode[],
        links: [] as TopologyLink[],
        loading: false,
        // 视图状态
        viewState: {
            scale: 1,
            offset: { x: 0, y: 0 }
        },
        // UI 状态控制
        selectedNodeId: null as string | number | null,
        isDetailPanelOpen: false,
        // 新增：单点实时监控数据
        // 节点深度监控模拟数据（初始化 20 个点）
        nodeMetrics: {
            cpu_trend: [22, 25, 21, 28, 32, 35, 33, 30, 28, 26, 25, 28, 30, 32, 35, 38, 40, 38, 35, 33],
            mem_trend: [45, 45, 46, 46, 46, 47, 47, 48, 48, 48, 48, 49, 49, 49, 50, 50, 51, 51, 51, 52],
            net_tx: [45, 52, 48, 60, 85, 92, 78, 65, 55, 62, 70, 88, 95, 82, 75, 68, 60, 55, 50, 58],
            net_rx: [12, 15, 14, 18, 25, 30, 28, 22, 19, 21, 24, 28, 32, 29, 26, 23, 20, 18, 16, 19],
            timestamps: [
                '14:00:02', '14:00:04', '14:00:06', '14:00:08', '14:00:10',
                '14:00:12', '14:00:14', '14:00:16', '14:00:18', '14:00:20',
                '14:00:22', '14:00:24', '14:00:26', '14:00:28', '14:00:30',
                '14:00:32', '14:00:34', '14:00:36', '14:00:38', '14:00:40'
            ]
        }
    }),

    actions: {
        async fetchTopology() {
            this.loading = true
            try {
                // 对接后端接口: GET /api/v1/ws/:id/topology
                // const res = await axios.get(`/api/v1/ws/${workspaceId}/topology`)
                // this.nodes = res.data.nodes
                // this.links = res.data.links
                this.generateMockTopology()
            } finally {
                this.loading = false
            }
        },

        selectNode(id: string | number) {
            this.selectedNodeId = id
            this.isDetailPanelOpen = true
            // 每次选中新节点，可以在这里重置或获取该节点的特定数据
            // this.fetchNodeDetailMetrics(id)
        },

// 模拟数据动态推进（心跳更新）
        async tickNodeMetrics() {
            const now = new Date()
            const timeStr = now.toLocaleTimeString([], { hour12: false, minute: '2-digit', second: '2-digit' })

            // 移除旧数据
            this.nodeMetrics.timestamps.shift()
            this.nodeMetrics.cpu_trend.shift()
            this.nodeMetrics.net_tx.shift()
            this.nodeMetrics.net_rx.shift()
            this.nodeMetrics.mem_trend.shift()

            // 压入符合逻辑波动的新数据
            this.nodeMetrics.timestamps.push(timeStr)
            this.nodeMetrics.cpu_trend.push(Math.floor(Math.random() * 15 + 25))
            this.nodeMetrics.net_tx.push(Math.floor(Math.random() * 40 + 40))
            this.nodeMetrics.net_rx.push(Math.floor(Math.random() * 20 + 10))
            this.nodeMetrics.mem_trend.push(50 + Math.floor(Math.random() * 2))
        },
      

        generateMockTopology() {
            // 模拟节点数据 (基于你 840x520 的画布)
            this.nodes = [
                { id: 'n1', name: 'Center-Gateway', ip: '10.24.0.1', x: 420, y: 260, status: 'online', type: 'relay' },
                { id: 'n2', name: 'Edge-Shanghai', ip: '10.24.0.5', x: 150, y: 120, status: 'online', type: 'edge' },
                { id: 'n3', name: 'Edge-Beijing', ip: '10.24.0.8', x: 690, y: 120, status: 'online', type: 'edge' },
                { id: 'n4', name: 'HK-User-iMac', ip: '10.24.5.12', x: 420, y: 450, status: 'offline', type: 'client' },
            ]

            // 模拟链路数据
            this.links = [
                { id: 'l1', from: 'n1', to: 'n2', quality: 'good', latency: 24, bandwidth: '1.2 Gbps' },
                { id: 'l2', from: 'n1', to: 'n3', quality: 'good', latency: 35, bandwidth: '800 Mbps' },
                { id: 'l3', from: 'n1', to: 'n4', quality: 'error', latency: 0, bandwidth: '0 Mbps' },
            ]
        },

        // 联动组件内的缩放逻辑
        updateZoom(delta: number) {
            const newScale = Math.min(2.2, Math.max(0.6, +(this.viewState.scale + delta).toFixed(2)))
            this.viewState.scale = newScale
        },

        resetView() {
            this.viewState.scale = 1
            this.viewState.offset = { x: 0, y: 0 }
        }
    }
})