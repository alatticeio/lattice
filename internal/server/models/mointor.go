package models

// Workspace-level response

// WorkspaceStats corresponds to the four core metric cards at the top
type WorkspaceStats struct {
	Throughput  float64 `json:"throughput"` // Mbps
	Latency     int64   `json:"latency"`    // ms
	LossRate    float64 `json:"loss_rate"`  // %
	ActiveLinks int     `json:"active_links"`
}

// NodeMonitorDetail corresponds to the table detail in the middle
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

// AggregatedMonitorResponse is the final response object returned to the frontend
type AggregatedMonitorResponse struct {
	WorkspaceID string              `json:"workspace_id"`
	LiveStats   []StatCard          `json:"live_stats"`
	Nodes       []NodeMonitorDetail `json:"nodes"`
	Events      []EventLog          `json:"events"`
	Trend       TrendData           `json:"trend"` // For area waveform chart
}

// StatCard corresponds to the four small cards at the top of the frontend
type StatCard struct {
	Label   string `json:"label"`   // e.g. "Real-time Throughput"
	Value   string `json:"value"`   // e.g. "124.8"
	Unit    string `json:"unit"`    // e.g. "Mbps"
	Trend   string `json:"trend"`   // "up", "down", "stable"
	Color   string `json:"color"`   // e.g. "text-blue-500"
	Percent int    `json:"percent"` // Progress bar percentage
}

// EventLog corresponds to the event stream / audit log at the bottom
type EventLog struct {
	Time   string `json:"time"`  // Formatted time: "14:20:01"
	Level  string `json:"level"` // "info", "warn", "error"
	Msg    string `json:"msg"`   // Log message content
	WSName string `json:"ws"`    // Associated workspace name (useful in global mode)
	Tone   string `json:"tone"`  // Corresponds to frontend color: "emerald", "amber", "blue"
}

// TrendData corresponds to the area waveform chart in the middle
// For charting, the backend needs to return a set of time-series data
type TrendData struct {
	Timestamps []string  `json:"timestamps"` // X-axis: ["10:00", "10:05", ...]
	TXData     []float64 `json:"tx_data"`    // Y-axis 1: transmit rate
	RXData     []float64 `json:"rx_data"`    // Y-axis 2: receive rate
}
