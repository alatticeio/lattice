import { defineStore } from 'pinia'
import { getGlobalDashboard, getWorkspaceDashboard } from '@/api/dashboard'
import type {
    GlobalStatItem,
    GlobalEventItem,
    NodeMonitorDetail,
    TrendData,
    WorkspaceDashboardResponse,
    NodeCPUItem,
} from '@/types/monitor'

const POLL_INTERVAL = 30_000 // 30s
const SPARKLINE_LEN = 12

function formatBytes(bytes: number): string {
    if (bytes >= 1e12) return `${(bytes / 1e12).toFixed(1)} TB`
    if (bytes >= 1e9)  return `${(bytes / 1e9).toFixed(1)} GB`
    if (bytes >= 1e6)  return `${(bytes / 1e6).toFixed(1)} MB`
    return `${bytes.toFixed(0)} B`
}

function makeSparkline(value: number, up: boolean): number[] {
    const base = value * 0.5
    return Array.from({ length: SPARKLINE_LEN }, (_, i) => {
        const progress = i / (SPARKLINE_LEN - 1)
        const noise = (Math.random() - 0.5) * value * 0.15
        return up
            ? base + (value - base) * progress + noise
            : value - (value - base) * progress + noise
    })
}

export const useDashboardStore = defineStore('dashboard', {
    state: () => ({
        loading: false,
        error: null as string | null,

        // ── Global dashboard ─────────────────────────────────────────────
        globalStats:  [] as GlobalStatItem[],
        globalEvents: [] as GlobalEventItem[],
        topNodes:     [] as NodeMonitorDetail[],
        globalTrend:  { timestamps: [], tx_data: [], rx_data: [] } as TrendData,

        // ── Workspace dashboard ──────────────────────────────────────────
        wsData:        null as WorkspaceDashboardResponse | null,
        wsLoading:     false,

        // ── Rolling sparkline state ──────────────────────────────────────
        txHistory: Array(30).fill(0) as number[],
        rxHistory: Array(30).fill(0) as number[],
        txRate: 0,
        rxRate: 0,

        _timer: null as ReturnType<typeof setInterval> | null,
    }),

    getters: {
        // ── Mode detection ────────────────────────────────────────────────
        activeWsID: (): string => localStorage.getItem('active_ws_id') ?? '',

        /** True when a workspace is active and workspace data has been loaded */
        isWorkspaceMode: (state): boolean =>
            !!(localStorage.getItem('active_ws_id') && state.wsData),

        // ── Global stat cards ─────────────────────────────────────────────
        statCards: (state) => {
            const iconNames = ['Server', 'Activity', 'ShieldCheck', 'AlertTriangle']
            return state.globalStats.slice(0, 4).map((s, i) => ({
                title:    s.label,
                value:    `${s.value} ${s.unit}`.trim(),
                change:   s.trend,
                trend:    s.trendUp ? ('up' as const) : ('down' as const),
                iconName: iconNames[i] ?? 'Server',
                sparkline: makeSparkline(parseFloat(s.value) || 1, s.trendUp),
            }))
        },

        // ── Workspace stat cards ──────────────────────────────────────────
        wsStatCards: (state) => {
            if (!state.wsData) return []
            const iconNames = ['Server', 'Activity', 'ShieldCheck', 'AlertTriangle']
            return state.wsData.stat_cards.map((s, i) => ({
                title:    s.label,
                value:    `${s.value} ${s.unit}`.trim(),
                change:   s.trend_pct || s.trend,
                trend:    (s.trend === 'down') ? ('down' as const) : ('up' as const),
                iconName: iconNames[i] ?? 'Server',
                sparkline: makeSparkline(parseFloat(s.value) || 1, s.trend !== 'down'),
            }))
        },

        // ── Effective display data (workspace when active, else global) ───

        displayStatCards: (state) => {
            const iconNames = ['Server', 'Activity', 'ShieldCheck', 'AlertTriangle']
            const isWsMode = !!(localStorage.getItem('active_ws_id') && state.wsData)
            if (isWsMode && state.wsData) {
                return state.wsData.stat_cards.map((s, i) => ({
                    title:    s.label,
                    value:    `${s.value} ${s.unit}`.trim(),
                    change:   s.trend_pct || s.trend,
                    trend:    (s.trend === 'down') ? ('down' as const) : ('up' as const),
                    iconName: iconNames[i] ?? 'Server',
                    sparkline: makeSparkline(parseFloat(s.value) || 1, s.trend !== 'down'),
                }))
            }
            return state.globalStats.slice(0, 4).map((s, i) => ({
                title:    s.label,
                value:    `${s.value} ${s.unit}`.trim(),
                change:   s.trend,
                trend:    s.trendUp ? ('up' as const) : ('down' as const),
                iconName: iconNames[i] ?? 'Server',
                sparkline: makeSparkline(parseFloat(s.value) || 1, s.trendUp),
            }))
        },

        displayTxData: (state): number[] => {
            const ws = state.wsData
            if (localStorage.getItem('active_ws_id') && ws?.throughput_trend.tx_data.length)
                return ws.throughput_trend.tx_data
            return state.globalTrend.tx_data.length ? state.globalTrend.tx_data : Array(6).fill(0)
        },

        displayRxData: (state): number[] => {
            const ws = state.wsData
            if (localStorage.getItem('active_ws_id') && ws?.throughput_trend.rx_data.length)
                return ws.throughput_trend.rx_data
            return state.globalTrend.rx_data.length ? state.globalTrend.rx_data : Array(6).fill(0)
        },

        displayTimeline: (state): string[] => {
            const ws = state.wsData
            if (localStorage.getItem('active_ws_id') && ws?.throughput_trend.timestamps.length)
                return ws.throughput_trend.timestamps
            return state.globalTrend.timestamps.length
                ? state.globalTrend.timestamps
                : ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00']
        },

        // ── Node load bar chart ───────────────────────────────────────────

        /** Workspace node CPU list (for bar chart) when active, else top global nodes by CPU */
        nodeLoadBar: (state): { name: string; load: number }[] => {
            const wsID = localStorage.getItem('active_ws_id')
            if (wsID && state.wsData?.node_cpu.length) {
                return [...state.wsData.node_cpu]
                    .sort((a: NodeCPUItem, b: NodeCPUItem) => b.cpu - a.cpu)
                    .slice(0, 5)
                    .map((n: NodeCPUItem) => ({ name: n.name || n.peer_id, load: Math.round(n.cpu) }))
            }
            return [...state.topNodes]
                .sort((a, b) => b.cpu - a.cpu)
                .slice(0, 5)
                .map(n => ({ name: n.name, load: Math.round(n.cpu) }))
        },

        // ── High-traffic nodes table ──────────────────────────────────────

        topTrafficNodes: (state) => {
            const wsID = localStorage.getItem('active_ws_id')
            const nodes = (wsID && state.wsData?.top_nodes.length)
                ? state.wsData.top_nodes
                : state.topNodes
            return [...nodes]
                .sort((a, b) => b.total_tx - a.total_tx)
                .slice(0, 5)
                .map(n => ({
                    name:    n.name,
                    ip:      n.endpoint || '—',
                    traffic: formatBytes(n.total_tx + n.total_rx),
                    load:    Math.round(n.cpu),
                    status:  n.online ? 'Healthy' : 'Offline',
                }))
        },

        // ── Audit log entries ─────────────────────────────────────────────
        auditLogs: (state) =>
            state.globalEvents.slice(0, 10).map(e => ({
                time:   e.time,
                user:   e.ws || 'System',
                action: e.type,
                target: e.content,
                tone:   e.tone || 'blue',
            })),

        // ── Legacy getters (kept for compatibility) ───────────────────────
        chartTimeline: (state): string[] => {
            const ws = state.wsData
            if (localStorage.getItem('active_ws_id') && ws?.throughput_trend.timestamps.length)
                return ws.throughput_trend.timestamps
            return state.globalTrend.timestamps.length
                ? state.globalTrend.timestamps
                : ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00']
        },
        txChartData: (state): number[] => {
            const ws = state.wsData
            if (localStorage.getItem('active_ws_id') && ws?.throughput_trend.tx_data.length)
                return ws.throughput_trend.tx_data
            return state.globalTrend.tx_data.length ? state.globalTrend.tx_data : Array(6).fill(0)
        },
        rxChartData: (state): number[] => {
            const ws = state.wsData
            if (localStorage.getItem('active_ws_id') && ws?.throughput_trend.rx_data.length)
                return ws.throughput_trend.rx_data
            return state.globalTrend.rx_data.length ? state.globalTrend.rx_data : Array(6).fill(0)
        },
    },

    actions: {
        /** Fetch global dashboard data */
        async fetch() {
            this.loading = true
            this.error = null
            try {
                const res = await getGlobalDashboard()
                const d = res.data
                this.globalStats  = d.global_stats  ?? []
                this.globalEvents = d.global_events ?? []
                this.topNodes     = d.top_nodes      ?? []
                const rawTrend    = d.global_trend   ?? {}
                this.globalTrend  = {
                    timestamps: rawTrend.timestamps ?? [],
                    tx_data:    rawTrend.tx_data    ?? [],
                    rx_data:    rawTrend.rx_data    ?? [],
                }

                const txLast = this.globalTrend.tx_data.at(-1) ?? 0
                const rxLast = this.globalTrend.rx_data.at(-1) ?? 0
                if (!localStorage.getItem('active_ws_id')) {
                    this.txRate = txLast
                    this.rxRate = rxLast
                }
            } catch (e: any) {
                this.error = e?.message ?? 'Failed to load dashboard'
            } finally {
                this.loading = false
            }
        },

        /** Fetch workspace-scoped dashboard data */
        async fetchWorkspace() {
            const wsID = localStorage.getItem('active_ws_id')
            if (!wsID) return
            this.wsLoading = true
            try {
                const res = await getWorkspaceDashboard(wsID)
                const d = res.data
                const rawTrend = d.throughput_trend ?? {}
                // Normalize null slices (Go nil slice → JSON null)
                this.wsData = {
                    stat_cards: d.stat_cards ?? [],
                    node_cpu:   d.node_cpu   ?? [],
                    top_nodes:  d.top_nodes  ?? [],
                    throughput_trend: {
                        timestamps: rawTrend.timestamps ?? [],
                        tx_data:    rawTrend.tx_data    ?? [],
                        rx_data:    rawTrend.rx_data    ?? [],
                    },
                }
                const tx = this.wsData.throughput_trend.tx_data.at(-1) ?? 0
                const rx = this.wsData.throughput_trend.rx_data.at(-1) ?? 0
                this.txRate = tx
                this.rxRate = rx
            } catch {
                // Non-fatal: workspace may not have telemetry yet
            } finally {
                this.wsLoading = false
            }
        },

        tick() {
            const txNoise = this.txRate * 0.1 * (Math.random() - 0.5)
            const rxNoise = this.rxRate * 0.1 * (Math.random() - 0.5)
            const tx = Math.max(0, this.txRate + txNoise)
            const rx = Math.max(0, this.rxRate + rxNoise)
            this.txHistory.push(tx)
            this.txHistory.shift()
            this.rxHistory.push(rx)
            this.rxHistory.shift()
        },

        startPolling() {
            this.fetch()
            this.fetchWorkspace()
            if (this._timer) return
            this._timer = setInterval(() => {
                this.fetch()
                this.fetchWorkspace()
            }, POLL_INTERVAL)
            setInterval(() => this.tick(), 2000)
        },

        stopPolling() {
            if (this._timer) {
                clearInterval(this._timer)
                this._timer = null
            }
        },
    },
})
