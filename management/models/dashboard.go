package models

// WorkspaceDashboardResponse 工作空间维度 Dashboard 数据
type WorkspaceDashboardResponse struct {
	StatCards       []WorkspaceStatCard `json:"stat_cards"`
	ThroughputTrend TrendData           `json:"throughput_trend"`
	NodeCPU         []NodeCPUItem       `json:"node_cpu"`
	TopNodes        []NodeMonitorDetail `json:"top_nodes"`
}

// WorkspaceStatCard 工作空间顶部指标卡片
type WorkspaceStatCard struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Unit     string `json:"unit"`
	Trend    string `json:"trend"`     // "up" | "down" | "stable"
	TrendPct string `json:"trend_pct"` // e.g. "+5%", can be empty
	Color    string `json:"color"`     // Tailwind class, e.g. "text-emerald-500"
}

// NodeCPUItem 节点 CPU/Memory 明细，用于 Node Load 柱状图
type NodeCPUItem struct {
	PeerID   string  `json:"peer_id"`
	Name     string  `json:"name"`
	CPU      float64 `json:"cpu"`       // percentage 0–100
	MemoryMB float64 `json:"memory_mb"` // megabytes
}

// DashboardResponse 全域视角返回数据
type DashboardResponse struct {
	GlobalStats    []GlobalStatItem    `json:"global_stats"`
	WorkspaceUsage []WorkspaceUsageRow `json:"workspace_usage"`
	GlobalEvents   []GlobalEventItem   `json:"global_events"`
	GlobalTrend    TrendData           `json:"global_trend"` // 24h 吞吐趋势（4h 粒度）
	TopNodes       []NodeMonitorDetail `json:"top_nodes"`    // Top 10 节点（按 24h 流量）
}

type GlobalStatItem struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Unit     string `json:"unit"`
	Trend    string `json:"trend"`
	Color    string `json:"color"`
	BarWidth string `json:"barWidth"`
	TrendUp  bool   `json:"trendUp"`
}

type WorkspaceUsageRow struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Nodes   int    `json:"nodes"`
	Traffic string `json:"traffic"`
	Health  int    `json:"health"`
	Status  string `json:"status"`
}

type GlobalEventItem struct {
	Time    string `json:"time"`
	WS      string `json:"ws"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Tone    string `json:"tone"` // 映射前端色值类
}
