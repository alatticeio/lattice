package models

const (
	// Nodes
	LATTICE_PEER_STATUS              = "lattice_peer_status"
	LATTICE_PEER_LATENCY_MS          = "lattice_peer_latency_ms"
	LATTICE_PEER_PACKET_LOSS_PERCENT = "lattice_peer_packet_loss_percent"
	WIREWFLOW_NODE_CPU_USEAGE        = "lattice_node_cpu_useage"
	LATTICE_NODE_UPTIME_SECONDS      = "lattice_node_uptime_seconds"
	LATTICE_NODE_MEMORY_BYTES        = "lattice_node_memory_bytes"

	LATTICE_PEER_TRAFFIC_BYTES_TOTAL = "lattice_peer_traffic_bytes_total"

	LATTICE_PEER_HANDSHAKE_TIME_MS = "lattice_peer_handshake_time_ms"
)

// NodeSnapshot corresponds to the frontend entity
type NodeSnapshot struct {
	ID          string `json:"id" gorm:"primaryKey"`
	Name        string `json:"name"`
	IP          string `json:"ip"`
	Status      string `json:"status"`       // "online" | "offline"
	HealthLevel string `json:"health_level"` // "success" | "warning" | "error"
	// Metrics stores formatted strings (e.g. "5%")
	Metrics map[string]string `json:"metrics" gorm:"serializer:json"`
	// RawMetrics stores raw numeric values (for frontend charting)
	RawMetrics  map[string]float64 `json:"raw_metrics" gorm:"serializer:json"`
	X           float64            `json:"x"`
	Y           float64            `json:"y"`
	WorkspaceID string             `json:"-"` // Tenant isolation
}
