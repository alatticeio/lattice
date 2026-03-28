// types/monitor.ts

export interface NodeSnapshot {
    id: string;
    name: string;
    ip: string;
    status: 'online' | 'offline';
    health_level: 'success' | 'warning' | 'error';
    metrics: Record<string, string>; // 动态 Map，如 { "cpu": "5%", "uptime": "2h" }
    raw_metrics: Record<string, number>;
    x?: number;
    y?: number;
}

export interface WorkspaceResponse {
    code: number;
    data: NodeSnapshot[];
    events: any[]; // 如果有事件类型也可以细化
}