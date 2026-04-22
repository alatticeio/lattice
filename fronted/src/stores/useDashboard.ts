import { defineStore } from 'pinia'
import { getGlobalDashboard } from '@/api/dashboard'
import type { GlobalStatItem, GlobalEventItem, NodeMonitorDetail, TrendData } from '@/types/monitor'

const POLL_INTERVAL = 30_000 // 30s
const SPARKLINE_LEN = 12

function formatBytes(bytes: number): string {
    if (bytes >= 1e12) return `${(bytes / 1e12).toFixed(1)} TB`
    if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`
    if (bytes >= 1e6) return `${(bytes / 1e6).toFixed(1)} MB`
    return `${bytes.toFixed(0)} B`
}

// Generate a simple sparkline trending toward `value`
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

        // API data
        globalStats: [] as GlobalStatItem[],
        globalEvents: [] as GlobalEventItem[],
        topNodes: [] as NodeMonitorDetail[],
        globalTrend: { timestamps: [], tx_data: [], rx_data: [] } as TrendData,

        // Rolling real-time sparkline (updated by tick)
        txHistory: Array(30).fill(0) as number[],
        rxHistory: Array(30).fill(0) as number[],
        txRate: 0,
        rxRate: 0,

        _timer: null as ReturnType<typeof setInterval> | null,
    }),

    getters: {
        // Map GlobalStatItem → stat card shape expected by dashboard/index.vue
        statCards: (state) => {
            const iconNames = ['Server', 'Activity', 'ShieldCheck', 'AlertTriangle']
            return state.globalStats.slice(0, 4).map((s, i) => ({
                title: s.label,
                value: `${s.value} ${s.unit}`.trim(),
                change: s.trend,
                trend: s.trendUp ? ('up' as const) : ('down' as const),
                iconName: iconNames[i] ?? 'Server',
                sparkline: makeSparkline(parseFloat(s.value) || 1, s.trendUp),
            }))
        },

        // Top 5 nodes sorted by CPU for the bar chart
        nodeLoadBar: (state) =>
            [...state.topNodes]
                .sort((a, b) => b.cpu - a.cpu)
                .slice(0, 5)
                .map(n => ({
                    name: n.name,
                    load: Math.round(n.cpu),
                })),

        // Top 5 nodes sorted by traffic for the table
        topTrafficNodes: (state) =>
            [...state.topNodes]
                .sort((a, b) => b.total_tx - a.total_tx)
                .slice(0, 5)
                .map(n => ({
                    name: n.name,
                    ip: n.endpoint || '—',
                    traffic: formatBytes(n.total_tx + n.total_rx),
                    load: Math.round(n.cpu),
                    status: n.online ? 'Healthy' : 'Offline',
                })),

        // Audit log entries mapped to dashboard tone colours
        auditLogs: (state) =>
            state.globalEvents.slice(0, 10).map(e => ({
                time: e.time,
                user: e.ws || 'System',
                action: e.type,
                target: e.content,
                tone: e.tone || 'blue',
            })),

        chartTimeline: (state): string[] =>
            state.globalTrend.timestamps.length > 0
                ? state.globalTrend.timestamps
                : ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00'],

        txChartData: (state): number[] =>
            state.globalTrend.tx_data.length > 0
                ? state.globalTrend.tx_data
                : Array(6).fill(0),

        rxChartData: (state): number[] =>
            state.globalTrend.rx_data.length > 0
                ? state.globalTrend.rx_data
                : Array(6).fill(0),
    },

    actions: {
        async fetch() {
            this.loading = true
            this.error = null
            try {
                const res = await getGlobalDashboard()
                const d = res.data
                this.globalStats = d.global_stats ?? []
                this.globalEvents = d.global_events ?? []
                this.topNodes = d.top_nodes ?? []
                this.globalTrend = d.global_trend ?? { timestamps: [], tx_data: [], rx_data: [] }

                // Seed rolling sparkline from latest trend point
                const txLast = this.globalTrend.tx_data.at(-1) ?? 0
                const rxLast = this.globalTrend.rx_data.at(-1) ?? 0
                this.txRate = txLast
                this.rxRate = rxLast
            } catch (e: any) {
                this.error = e?.message ?? 'Failed to load dashboard'
            } finally {
                this.loading = false
            }
        },

        tick() {
            // Keep sparklines alive between API polls
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
            if (this._timer) return
            // Poll API every 30s
            this._timer = setInterval(() => this.fetch(), POLL_INTERVAL)
            // Animate sparklines every 2s
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
