package models

// 对应空间级别的返回响应

// WorkspaceStats 对应顶部的四个核心指标卡片
type WorkspaceStats struct {
	Throughput  float64 `json:"throughput"` // Mbps
	Latency     int64   `json:"latency"`    // ms
	LossRate    float64 `json:"loss_rate"`  // %
	ActiveLinks int     `json:"active_links"`
}

// NodeMonitorDetail 对应中间的表格明细
type NodeMonitorDetail struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	VIP            string  `json:"vip"`
	ConnectionType string  `json:"connection_type"` // p2p, relay
	Endpoint       string  `json:"endpoint"`
	LastHandshake  int64   `json:"last_handshake"`
	TotalRx        int64   `json:"total_rx"`
	TotalTx        int64   `json:"total_tx"`
	CurrentRate    float64 `json:"current_rate"`
	Online         bool    `json:"online"`
	CPU            float64 `json:"cpu"`
	Memory         float64 `json:"memory"`
}

// AggregatedMonitorResponse 最终返回给前端的单一对象
type AggregatedMonitorResponse struct {
	WorkspaceID string              `json:"workspace_id"`
	LiveStats   []StatCard          `json:"live_stats"`
	Nodes       []NodeMonitorDetail `json:"nodes"`
	Events      []EventLog          `json:"events"`
	Trend       TrendData           `json:"trend"` // 用于面积波形图
}

// StatCard 对应前端顶部的四个小卡片
type StatCard struct {
	Label   string `json:"label"`   // 例如: "实时吞吐"
	Value   string `json:"value"`   // 例如: "124.8"
	Unit    string `json:"unit"`    // 例如: "Mbps"
	Trend   string `json:"trend"`   // "up", "down", "stable"
	Color   string `json:"color"`   // 例如: "text-blue-500"
	Percent int    `json:"percent"` // 进度条百分比
}

// EventLog 对应底部的事件流/审计日志
type EventLog struct {
	Time   string `json:"time"`  // 格式化后的时间: "14:20:01"
	Level  string `json:"level"` // "info", "warn", "error"
	Msg    string `json:"msg"`   // 日志内容
	WSName string `json:"ws"`    // 所属工作空间名称 (全局模式下有用)
	Tone   string `json:"tone"`  // 对应前端颜色: "emerald", "amber", "blue"
}

// TrendData 对应中间的面积波形图
// 为了绘图，后端需要返回一组时间序列数据
type TrendData struct {
	Timestamps []string  `json:"timestamps"` // X轴：["10:00", "10:05", ...]
	TXData     []float64 `json:"tx_data"`    // Y轴1：发送速率
	RXData     []float64 `json:"rx_data"`    // Y轴2：接收速率
}
