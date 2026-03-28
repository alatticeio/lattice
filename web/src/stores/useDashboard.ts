import { defineStore } from 'pinia'

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
                // 实际对接后端接口时：
                // const res = await axios.get('/api/v1/global/dashboard')
                // this.data = res.data
                this.generateMockGlobalData()
            } catch (err) {
                console.error('Fetch global dashboard failed', err)
            } finally {
                this.loading = false
            }
        },

        generateMockGlobalData() {
            const now = new Date()
            this.data.last_updated = now.toLocaleTimeString([], { hour12: false, minute: '2-digit', second: '2-digit' })

            // 1. 模拟全域指标
            this.data.global_stats = [
                { label: '活跃工作空间', value: '12', unit: 'SETS', trend: '+1', color: 'text-blue-500', barWidth: '50%', trendUp: true },
                { label: '全网在线节点', value: '1,284', unit: 'NODE', trend: 'Live', color: 'text-emerald-500', barWidth: '75%', trendUp: true },
                { label: '全域总吞吐', value: '8.4', unit: 'BAND', trend: 'Gbps', color: 'text-primary', barWidth: '66%', trendUp: true },
                { label: '未处理告警', value: '03', unit: 'WARN', trend: 'Critical', color: 'text-error', barWidth: '25%', trendUp: false },
            ]

            // 2. 模拟工作空间排行
            this.data.workspace_usage = [
                { name: '上海 AIGC 算力池', type: 'Production', nodes: 450, traffic: '2.4 TB', health: 98, status: 'Running' },
                { name: '北京研发中心', type: 'Internal', nodes: 320, traffic: '1.8 TB', health: 100, status: 'Running' },
                { name: '香港跨境加速', type: 'Edge', nodes: 180, traffic: '3.2 TB', health: 92, status: 'Warning' },
                { name: '内蒙边缘节点', type: 'Production', nodes: 120, traffic: '450 GB', health: 99, status: 'Running' },
            ]

            // 3. 模拟全域风险审计 (tone 对应不同语义颜色)
            this.data.global_events = [
                { time: '14:20:01', ws: '上海', type: 'Security', content: '检测到大流量跨域同步请求', tone: 'blue' },
                { time: '14:15:22', ws: '香港', type: 'Node', content: '3个边缘节点由于延迟过高切换至中继模式', tone: 'amber' },
                { time: '13:50:10', ws: '系统', type: 'Auth', content: '管理员更新了全域防火墙白名单', tone: 'emerald' },
            ]
        }
    }
})