import { defineStore } from 'pinia'
import { getDashboardOverview } from '@/api/dashboard'

export const useUserDashboardStore = defineStore('userDashboard', {
    state: () => ({
        data: {
            // 1. 全域宏观指标 (Global Metrics)
            global_stats: [] as { label: string; value: string; unit: string; trend: string; color: string; barWidth: string; trendUp: boolean }[],

            // 2. 工作空间排行榜 (Workspace Rankings)
            workspace_usage: [] as { name: string; type: string; nodes: number; traffic: string; health: number; status: string }[],

            // 3. 全域风险审计 (Global Audit Events)
            global_events: [] as { time: string; ws: string; type: string; content: string; tone: string }[],

            // 全域刷新状态
            last_updated: ''
        },
        loading: false
    }),

    actions: {
        async refresh() {
            this.loading = true
            try {
                const res = await getDashboardOverview()
                const now = new Date()
                this.data.last_updated = now.toLocaleTimeString([], { hour12: false, minute: '2-digit', second: '2-digit' })
                this.data.global_stats = res.data.global_stats ?? []
                this.data.workspace_usage = res.data.workspace_usage ?? []
                this.data.global_events = res.data.global_events ?? []
            } catch (err) {
                console.error('Fetch global dashboard failed', err)
            } finally {
                this.loading = false
            }
        }
    }
})