import {defineStore} from 'pinia'
import axios from 'axios'
import {getSnapshot} from "@/api/monitor";

export const useMonitorStore = defineStore('monitor', {
    state: () => ({
        // 核心监控数据
        data: {
            live_stats: [] as any[],
            trend: {
                timestamps: [] as string[],
                tx_data: [] as number[],
                rx_data: [] as number[]
            },
            // 新增：链路质量排行
            latency_rank: [] as { name: string; target: string; latency: number; percent: number; status: string }[],
            // 新增：实时事件流
            events: [] as { time: string; level: string; msg: string; tone: string }[],
            // 节点详细数据
            nodes: [] as {
                id: number | string;
                name: string;
                peerName: string;  //对端peer
                vip: string;
                connectionType: string;
                endpoint: string;
                lastHandshake: string;
                totalRx: string;
                totalTx: string;
                currentRate: string;
                online: boolean;
                cpu: number;
                mem: number;
            }[]
        },
        isLive: true
    }),
    actions: {
        async refresh() {
            try {
                // const res = await getSnapshot()
                // this.data = res.data
                // 演示：这里先生成模拟数据逻辑
                this.generateMockData()
            } catch (err) {
                console.error('Fetch monitor data failed', err)
            }
        },

        generateMockData() {
            const now = new Date()
            const timeStr = now.toLocaleTimeString([], { hour12: false, minute: '2-digit', second: '2-digit' })

            // 1. 更新卡片数据
            this.data.live_stats = [
                { label: '实时吞吐', value: (100 + Math.random() * 50).toFixed(1), unit: 'Mbps', trend: 'up', color: 'text-blue-500', percent: 65 },
                { label: '平均延迟', value: (20 + Math.random() * 10).toFixed(0), unit: 'ms', trend: 'down', color: 'text-emerald-500', percent: 20 },
                { label: '丢包率', value: (Math.random() * 0.1).toFixed(2), unit: '%', trend: 'stable', color: 'text-amber-500', percent: 5 },
                { label: '活动隧道', value: '18', unit: 'Links', trend: 'up', color: 'text-indigo-500', percent: 80 },
            ]

            // 2. 更新趋势图（维持一个 30 个点的滑动窗口）
            if (this.data.trend.timestamps.length > 30) {
                this.data.trend.timestamps.shift()
                this.data.trend.tx_data.shift()
                this.data.trend.rx_data.shift()
            }

            this.data.trend.timestamps.push(timeStr)
            this.data.trend.tx_data.push(Math.floor(Math.random() * 40 + 60))
            this.data.trend.rx_data.push(Math.floor(Math.random() * 30 + 40))

            // 模拟链路排行数据
            this.data.latency_rank = [
                { name: 'Edge-SH', target: 'Gateway', latency: 24, percent: 30, status: 'emerald' },
                { name: 'HK-Office', target: 'Relay-01', latency: 48, percent: 55, status: 'emerald' },
                { name: 'AWS-BJ', target: 'Gateway', latency: 156, percent: 90, status: 'amber' },
            ].sort((a, b) => a.latency - b.latency) // 按延迟从小到大排

            // 这里的 tone 字段由后端判定：info -> blue, warn -> amber, error -> red
            this.data.events = [
                { time: new Date().toLocaleTimeString(), level: 'info', msg: '节点 edge-sh-prod-01 握手成功 (P2P)', tone: 'blue' },
                { time: new Date().toLocaleTimeString(), level: 'warn', msg: '链路 hk-office -> relay 延迟波动 > 150ms', tone: 'amber' },
                { time: new Date().toLocaleTimeString(), level: 'info', msg: '工作空间全局路由表已更新', tone: 'blue' },
            ]

            // 模拟节点详细数据
            this.data.nodes = [
                {
                    id: 1,
                    name: 'edge-sh-prod-01',
                    peerName: "edge-sh-prod-02",
                    vip: '10.24.0.5',
                    connectionType: 'p2p',
                    endpoint: '221.23.45.102:51820',
                    lastHandshake: '12s',
                    totalRx: '1.42 GB',
                    totalTx: '842 MB',
                    currentRate: '4.2 Mb/s',
                    online: true,
                    cpu: 42,
                    mem: 18
                },
                {
                    id: 2,
                    name: 'hk-office-imac',
                    peerName: 'hk-office-linux',
                    vip: '10.24.0.12',
                    connectionType: 'relay',
                    endpoint: 'Relay: HK-Global-01',
                    lastHandshake: '4m 22s',
                    totalRx: '245 MB',
                    totalTx: '120 MB',
                    currentRate: '128 Kb/s',
                    online: true,
                    cpu: 12,
                    mem: 65
                },
                {
                    id: 3,
                    name: 'aws-bj-node',
                    peerName: 'aws-bj-node1',
                    vip: '10.24.5.1',
                    connectionType: 'p2p',
                    endpoint: '54.12.109.5:51820',
                    lastHandshake: '---',
                    totalRx: '0 B',
                    totalTx: '0 B',
                    currentRate: '0 b/s',
                    online: false,
                    cpu: 0,
                    mem: 0
                }
            ]
        }
    }

})