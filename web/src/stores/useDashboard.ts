import { defineStore } from 'pinia'

// ── Types ─────────────────────────────────────────────────────────────
export interface MetricCard {
  label: string
  value: string
  subValue?: string
  icon: string          // lucide icon name
  tone: 'emerald' | 'blue' | 'violet' | 'amber'
  barValue: number      // 0-100
  trend: string
  trendUp: boolean
}

export interface TopoNode {
  id: string
  name: string
  x: number
  y: number
  status: 'online' | 'offline' | 'relay'
}

export interface TopoLink {
  source: string
  target: string
  quality: number
  type: 'p2p' | 'relay'
}

export interface WorkspaceCard {
  name: string
  slug: string
  status: 'active' | 'idle' | 'error'
  nodeUsed: number
  nodeTotal: number
  health: number
}

export interface NetworkEvent {
  time: string
  type: 'handshake' | 'join' | 'leave' | 'token' | 'warning'
  content: string
  ws: string
  tone: 'emerald' | 'amber' | 'rose' | 'blue'
}

// ── Store ──────────────────────────────────────────────────────────────
const MAX_POINTS = 30

export const useUserDashboardStore = defineStore('userDashboard', {
  state: () => ({
    loading: false,

    // Section 2: Metric cards
    metrics: [
      {
        label: '在线节点',
        value: '4 / 6',
        icon: 'Cpu',
        tone: 'emerald',
        barValue: 67,
        trend: '+1 今日',
        trendUp: true,
      },
      {
        label: '活跃工作空间',
        value: '4',
        icon: 'Server',
        tone: 'blue',
        barValue: 80,
        trend: '稳定',
        trendUp: true,
      },
      {
        label: '网络成员',
        value: '5',
        icon: 'Users',
        tone: 'violet',
        barValue: 50,
        trend: '+2 本周',
        trendUp: true,
      },
      {
        label: '有效 TOKEN',
        value: '4',
        icon: 'KeyRound',
        tone: 'amber',
        barValue: 67,
        trend: '1 即将过期',
        trendUp: false,
      },
    ] as MetricCard[],

    // Section 3: Mini topology
    topoNodes: [
      { id: 'alpha', name: 'node-alpha', x: 220, y: 90, status: 'online' },
      { id: 'beta', name: 'node-beta', x: 370, y: 160, status: 'online' },
      { id: 'gamma', name: 'node-gamma', x: 80, y: 170, status: 'offline' },
      { id: 'delta', name: 'node-delta', x: 300, y: 270, status: 'online' },
      { id: 'epsilon', name: 'node-epsilon', x: 150, y: 260, status: 'relay' },
      { id: 'relay1', name: 'relay-01', x: 220, y: 190, status: 'relay' },
    ] as TopoNode[],

    topoLinks: [
      { source: 'alpha', target: 'beta', quality: 95, type: 'p2p' },
      { source: 'alpha', target: 'gamma', quality: 0, type: 'relay' },
      { source: 'alpha', target: 'relay1', quality: 88, type: 'p2p' },
      { source: 'beta', target: 'delta', quality: 72, type: 'p2p' },
      { source: 'relay1', target: 'epsilon', quality: 60, type: 'relay' },
      { source: 'relay1', target: 'delta', quality: 45, type: 'relay' },
      { source: 'epsilon', target: 'gamma', quality: 0, type: 'relay' },
    ] as TopoLink[],

    // Section 3: Real-time traffic
    txHistory: Array(MAX_POINTS).fill(0) as number[],
    rxHistory: Array(MAX_POINTS).fill(0) as number[],
    txRate: 0,
    rxRate: 0,
    avgLatency: 0,

    // Section 4: Workspace overview (top 4)
    workspaces: [
      { name: 'Production', slug: 'prod-cluster', status: 'active', nodeUsed: 6, nodeTotal: 8, health: 98 },
      { name: 'Staging', slug: 'staging-env', status: 'active', nodeUsed: 3, nodeTotal: 4, health: 92 },
      { name: 'Dev Lab', slug: 'dev-lab', status: 'idle', nodeUsed: 1, nodeTotal: 4, health: 100 },
      { name: 'QA Network', slug: 'qa-net', status: 'error', nodeUsed: 2, nodeTotal: 4, health: 75 },
    ] as WorkspaceCard[],

    // Section 4: Network events
    events: [
      { time: '12:04:32', type: 'handshake', content: 'node-alpha 握手完成 (p2p)', ws: 'prod', tone: 'emerald' },
      { time: '12:04:15', type: 'join', content: '新成员 francis 加入工作空间', ws: 'staging', tone: 'blue' },
      { time: '12:03:58', type: 'token', content: 'Token api-key-007 被使用', ws: 'dev-lab', tone: 'amber' },
      { time: '12:03:42', type: 'leave', content: 'node-gamma 离线', ws: 'qa-net', tone: 'rose' },
      { time: '12:03:30', type: 'handshake', content: 'node-beta 重连 via relay', ws: 'prod', tone: 'amber' },
      { time: '12:02:58', type: 'join', content: 'node-epsilon p2p 连接建立', ws: 'prod', tone: 'emerald' },
    ] as NetworkEvent[],

    // Header summary counts
    workspaceCount: 4,
    nodeCount: 6,
    memberCount: 5,
  }),

  actions: {
    tick() {
      const tx = Math.random() * 8 + 1
      const rx = Math.random() * 6 + 0.5
      this.txHistory.push(tx)
      this.txHistory.shift()
      this.rxHistory.push(rx)
      this.rxHistory.shift()
      this.txRate = tx
      this.rxRate = rx
      this.avgLatency = Math.floor(Math.random() * 40 + 20)

      // Occasionally add a new network event
      if (Math.random() > 0.5) {
        const peers = ['node-alpha', 'node-beta', 'node-epsilon', 'node-zeta']
        const tones: NetworkEvent['tone'][] = ['emerald', 'amber', 'rose', 'blue']
        const types: NetworkEvent['type'][] = ['handshake', 'join', 'leave', 'token']
        const wsList = ['prod', 'staging', 'dev-lab', 'qa-net']
        const peer = peers[Math.floor(Math.random() * peers.length)]
        const type = types[Math.floor(Math.random() * types.length)]
        const tone = tones[Math.floor(Math.random() * tones.length)]
        const ws = wsList[Math.floor(Math.random() * wsList.length)]
        const now = new Date()
        const ts = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`
        const contents: Record<NetworkEvent['type'], string> = {
          handshake: `${peer} 握手完成`,
          join: `${peer} 已加入网络`,
          leave: `${peer} 已断开连接`,
          token: `Token 被 ${peer} 使用`,
          warning: `${peer} 延迟异常`,
        }
        this.events.unshift({ time: ts, type, content: contents[type], ws, tone })
        if (this.events.length > 20) this.events.pop()
      }
    },

    // kept for backward compatibility
    async refresh() {
      this.tick()
    },
  },
})
