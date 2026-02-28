package wireflow_exporter

import (
	"github.com/prometheus/client_golang/prometheus"
)

// 定义wireflow指标

var (
	// --- 1. 链路质量指标 (Gauges) ---
	PeerLatency = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_peer_latency_ms",
		Help: "RTT latency to peers in milliseconds",
	}, []string{"peer_id", "peer_name", "endpoint"})

	PeerLoss = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_peer_packet_loss_percent",
		Help: "Packet loss percentage to peers",
	}, []string{"peer_id"})

	// --- 2. 流量与负载指标 (Counters & Gauges) ---
	// 使用 Counter 记录累计流量，Grafana 中用 rate() 计算瞬时带宽
	PeerTrafficBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "wireflow_peer_traffic_bytes_total",
		Help: "Total bytes transferred per peer",
	}, []string{"peer_id", "direction"}) // direction: tx (发送), rx (接收)

	PeerPacketCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "wireflow_peer_packets_total",
		Help: "Total packets transferred per peer",
	}, []string{"peer_id", "direction"})

	// --- 3. 隧道状态 (Gauges) ---
	PeerStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_peer_status",
		Help: "Peer connection status (1: connected, 0: disconnected)",
	}, []string{"peer_id"})

	LastHandshakeTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wireflow_peer_last_handshake_seconds",
		Help: "Unix timestamp of the last successful handshake",
	}, []string{"peer_id"})

	// --- 4. 节点系统指标 (Gauges) ---
	NodeCpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "wireflow_node_cpu_usage_percent",
		Help: "CPU usage percentage of the wireflow process",
	})

	NodeMemUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "wireflow_node_memory_bytes",
		Help: "Memory usage of the wireflow process in bytes",
	})

	NodeUptime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "wireflow_node_uptime_seconds",
		Help: "Number of seconds since the wireflow process started",
	})
)

func init() {
	prometheus.MustRegister(
		PeerLatency, PeerLoss,
		PeerTrafficBytes, PeerPacketCount,
		PeerStatus, LastHandshakeTime,
		NodeCpuUsage, NodeMemUsage, NodeUptime,
	)
}
