package models

// TopologyResponse 拓扑视图响应
type TopologyResponse struct {
	Nodes []TopoNode `json:"nodes"`
	Links []TopoLink `json:"links"`
}

type TopoNode struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IP     string `json:"ip"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Status string `json:"status"` // online, offline
	Type   string `json:"type"`   // relay, edge, client
}

type TopoLink struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Quality string `json:"quality"` // good, warn, error
	Latency int    `json:"latency"`
}
