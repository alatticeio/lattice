package models

const (
	// 节点
	WIREFLOW_PEER_STATUS              = "wireflow_peer_status"
	WIREFLOW_PEER_LATENCY_MS          = "wireflow_peer_latency_ms"
	WIREFLOW_PEER_PACKET_LOSS_PERCENT = "wireflow_peer_packet_loss_percent"
	WIREWFLOW_NODE_CPU_USEAGE         = "wireflow_node_cpu_useage"
	WIREFLOW_NODE_UPTIME_SECONDS      = "wireflow_node_uptime_seconds"
	WIREFLOW_NODE_MEMORY_BYTES        = "wireflow_node_memory_bytes"

	WIREFLOW_PEER_TRAFFIC_BYTES_TOTAL = "wireflow_peer_traffic_bytes_total"

	WIREFLOW_PEER_HANDSHAKE_TIME_MS = "wireflow_peer_handshake_time_ms"
)

// NodeSnapshot 对应前端实体
type NodeSnapshot struct {
	ID          string `json:"id" gorm:"primaryKey"`
	Name        string `json:"name"`
	IP          string `json:"ip"`
	Status      string `json:"status"`       // "online" | "offline"
	HealthLevel string `json:"health_level"` // "success" | "warning" | "error"
	// Metrics 存放格式化后的字符串 (如 "5%")
	Metrics map[string]string `json:"metrics" gorm:"serializer:json"`
	// RawMetrics 存放原始数值 (用于前端绘图)
	RawMetrics  map[string]float64 `json:"raw_metrics" gorm:"serializer:json"`
	X           float64            `json:"x"`
	Y           float64            `json:"y"`
	WorkspaceID string             `json:"-"` // 租户隔离
}
