package models

const (
	// 节点
	LATTICE_PEER_STATUS              = "lattice_peer_status"
	LATTICE_PEER_LATENCY_MS          = "lattice_peer_latency_ms"
	LATTICE_PEER_PACKET_LOSS_PERCENT = "lattice_peer_packet_loss_percent"
	WIREWFLOW_NODE_CPU_USEAGE        = "lattice_node_cpu_useage"
	LATTICE_NODE_UPTIME_SECONDS      = "lattice_node_uptime_seconds"
	LATTICE_NODE_MEMORY_BYTES        = "lattice_node_memory_bytes"

	LATTICE_PEER_TRAFFIC_BYTES_TOTAL = "lattice_peer_traffic_bytes_total"

	LATTICE_PEER_HANDSHAKE_TIME_MS = "lattice_peer_handshake_time_ms"
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
