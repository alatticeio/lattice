package monitor

// PeerSnapshot 承载了拓扑图中一个点的所有信息
type PeerSnapshot struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Alias      string `json:"alias"`
	InternalIP string `json:"ip"`
	Status     string `json:"status"` // online, offline

	// 动态指标，统一存入这个 Map，由 autoFormat 函数填充
	// 比如：metrics["cpu"] = "15.2%", metrics["uptime"] = "1h 20m"
	Metrics map[string]string `json:"metrics"`

	// 视觉辅助字段
	HealthLevel string `json:"health_level"` // success, warn, error
}
