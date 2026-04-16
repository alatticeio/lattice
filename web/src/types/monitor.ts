// types/monitor.ts

export interface NodeSnapshot {
    id: string
    name: string
    ip: string
    status: 'online' | 'offline'
    health_level: 'success' | 'warning' | 'error'
    metrics: Record<string, string>
    raw_metrics: Record<string, number>
    x?: number
    y?: number
}

// ── Dashboard (Global) ────────────────────────────────────────────────

export interface GlobalStatItem {
    label: string
    value: string
    unit: string
    trend: string
    color: string
    barWidth: string
    trendUp: boolean
}

export interface WorkspaceUsageRow {
    name: string
    type: string
    nodes: number
    traffic: string
    health: number
    status: string
}

export interface GlobalEventItem {
    time: string
    ws: string
    type: string
    content: string
    tone: string
}

export interface TrendData {
    timestamps: string[]
    tx_data: number[]
    rx_data: number[]
}

export interface NodeMonitorDetail {
    id: string
    name: string
    vip: string
    connection_type: string
    endpoint: string
    last_handshake: number
    total_rx: number
    total_tx: number
    current_rate: number
    online: boolean
    cpu: number
    memory: number
}

export interface DashboardResponse {
    global_stats: GlobalStatItem[]
    workspace_usage: WorkspaceUsageRow[]
    global_events: GlobalEventItem[]
    global_trend: TrendData
    top_nodes: NodeMonitorDetail[]
}

// ── Workspace Monitor ─────────────────────────────────────────────────

export interface StatCard {
    label: string
    value: string
    unit: string
    trend: string
    color: string
    percent: number
}

export interface AggregatedMonitorResponse {
    workspace_id: string
    live_stats: StatCard[]
    nodes: NodeMonitorDetail[]
    events: EventLog[]
    trend: TrendData
}

export interface EventLog {
    time: string
    level: string
    msg: string
    ws: string
    tone: string
}

export interface WorkspaceResponse {
    code: number
    data: NodeSnapshot[]
    events: EventLog[]
}
