package internal

import (
	"github.com/prometheus/client_golang/prometheus"
)

// 定义wireflow指标

var (
	// --- 1. 链路质量指标 (Gauges) ---
	PeerLatency = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_peer_latency_ms",
		Help: "RTT latency to peers in milliseconds",
	}, []string{"workspace_id", "node_id", "peer_id", "peer_name", "endpoint"})

	PeerLoss = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_peer_packet_loss_percent",
		Help: "Packet loss percentage to peers",
	}, []string{"workspace_id", "node_id", "peer_id"})

	// --- 2. 流量与负载指标 (Counters & Gauges) ---
	// 使用 Counter 记录累计流量，Grafana 中用 rate() 计算瞬时带宽
	PeerTrafficBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "wireflow_peer_traffic_bytes_total",
		Help: "Total bytes transferred per peer",
	}, []string{"workspace_id", "node_id", "peer_id", "direction"}) // direction: tx (发送), rx (接收)

	PeerPacketCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "wireflow_peer_packets_total",
		Help: "Total packets transferred per peer",
	}, []string{"workspace_id", "node_id", "peer_id", "direction"})

	// --- 3. 隧道状态 (Gauges) ---
	PeerStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_peer_status",
		Help: "Peer connection status (1: connected, 0: disconnected)",
	}, []string{"workspace_id", "node_id", "peer_id", "endpoint", "alias"})

	LastHandshakeTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_peer_last_handshake_seconds",
		Help: "Unix timestamp of the last successful handshake",
	}, []string{"workspace_id", "node_id", "peer_id"})

	// --- 4. 节点系统指标 (Gauges) ---
	NodeCpuUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_node_cpu_usage_percent",
		Help: "CPU usage percentage of the wireflow process",
	}, []string{"workspace_id", "node_id"})

	// 各核心 CPU 使用率 (带 label)
	NodeCoreUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_node_core_usage_percent",
		Help: "Per-core CPU usage percentage",
	}, []string{"workspace_id", "node_id"}) // 这里的 label 叫 core_id

	NodeMemUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_node_memory_bytes",
		Help: "Memory usage of the wireflow process in bytes",
	}, []string{"workspace_id", "node_id"})

	NodeUptime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_node_uptime_seconds",
		Help: "Number of seconds since the wireflow process started",
	}, []string{"workspace_id", "node_id"})

	// 空间总数与利用率真
	WorkspaceResourceTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_workspace_resource_total",
		Help: "Workspace resource total of the wireflow process",
	}, []string{"workspace_id", "node_id", "resource_typ"}) //resource_type=nodes|bandwidth|subnets

	WorkspaceResourceUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_workspace_resource_usage",
		Help: "Workspace resource usage of the wireflow process",
	}, []string{"workspace_id", "node_id", "resource_typ"}) //resource_type=nodes|bandwidth|subnets

	// 在线节点
	WorkspaceNodeTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_workspace_nodes_total",
		Help: "",
	}, []string{"workspace_id", "node_id", "status"}) // status = online

	WorkspaceTunnels = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_workspace_tunnels",
		Help: "Number of tunnels in the workspace",
	}, []string{"workspace_id", "node_id", "status"}) //status="established|pending"

	// 新增：专门监控对等连接（Peering）的流量
	PeeringTraffic = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wireflow_peering_traffic_bytes_total",
			Help: "Traffic exchanged via peering between workspaces",
		},
		// local_ws: 本地空间, remote_ws: 对端空间, node_id: 哪个节点在跑流量
		[]string{"local_ws", "remote_ws", "node_id", "direction"},
	)
)

func init() {
	prometheus.MustRegister(
		PeerLatency, PeerLoss,
		PeerTrafficBytes, PeerPacketCount,
		PeerStatus, LastHandshakeTime,
		NodeCpuUsage, NodeCoreUsage, NodeMemUsage, NodeUptime, WorkspaceResourceUsage, WorkspaceTunnels, PeeringTraffic,
	)
}
