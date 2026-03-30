package monitor

import (
	"context"
	"sync"
	"time"
	"wireflow/internal"
	"wireflow/internal/infra"
	"wireflow/monitor/collector"
	wireflow_exporter "wireflow/monitor/wireflow-exporter"
)

// NodeIdentity holds the workspace and node labels applied to every metric emitted by this node.
type NodeIdentity struct {
	WorkspaceID string
	NodeID      string
}

// MetricWorker manages all metric collection goroutines for the local node.
type MetricWorker struct {
	stopChan chan struct{}
	identity NodeIdentity

	cpuCollector        collector.MetricCollector
	memCollector        collector.MetricCollector
	peerStatusCollector collector.MetricCollector
	peerManager         *infra.PeerManager
	startTime           time.Time

	// traffic delta tracking: WireGuard counters are cumulative,
	// so we only add the increment since the last sample to the Prometheus Counter.
	mu          sync.Mutex
	prevRxBytes map[string]float64 // peerID → last rx bytes
	prevTxBytes map[string]float64 // peerID → last tx bytes
}

func NewMetricWorker(identity NodeIdentity, peers *infra.PeerManager, ifName string) *MetricWorker {
	return &MetricWorker{
		stopChan:            make(chan struct{}),
		identity:            identity,
		cpuCollector:        collector.NewCPUCollector(),
		memCollector:        &collector.MemoryCollector{},
		peerStatusCollector: collector.NewPeerStatusCollector(peers, ifName),
		peerManager:         peers,
		startTime:           time.Now(),
		prevRxBytes:         make(map[string]float64),
		prevTxBytes:         make(map[string]float64),
	}
}

// StartLinkProbing probes each known peer's VIP with ICMP and records latency / packet loss.
func (mw *MetricWorker) StartLinkProbing(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				targets := mw.buildTargets()
				if len(targets) > 0 {
					wireflow_exporter.RunCycle(mw.identity.WorkspaceID, mw.identity.NodeID, targets)
				}
			case <-mw.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// buildTargets converts PeerManager entries into TargetPeer probing targets.
func (mw *MetricWorker) buildTargets() []wireflow_exporter.TargetPeer {
	peers := mw.peerManager.GetAll()
	targets := make([]wireflow_exporter.TargetPeer, 0, len(peers))
	for _, p := range peers {
		if p.Address == nil || *p.Address == "" {
			continue
		}
		targets = append(targets, *wireflow_exporter.NewTargetPeer(p.AppID, p.Name, *p.Address))
	}
	return targets
}

// StartSystemMetrics collects CPU, memory, and uptime metrics on a fixed interval.
func (mw *MetricWorker) StartSystemMetrics(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	wsID := mw.identity.WorkspaceID
	nodeID := mw.identity.NodeID
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// --- CPU ---
				if cpuMetrics, err := mw.cpuCollector.Collect(); err == nil {
					for _, m := range cpuMetrics {
						val, ok := m.Value().(float64)
						if !ok {
							continue
						}
						switch m.Name() {
						case "cpu_usage_total":
							internal.NodeCpuUsage.WithLabelValues(wsID, nodeID).Set(val)
						case "cpu_usage_core":
							internal.NodeCoreUsage.WithLabelValues(wsID, nodeID).Set(val)
						}
					}
				}

				// --- Memory ---
				if memMetrics, err := mw.memCollector.Collect(); err == nil {
					for _, m := range memMetrics {
						val, ok := m.Value().(float64)
						if !ok {
							continue
						}
						if m.Name() == "memory_used" {
							internal.NodeMemUsage.WithLabelValues(wsID, nodeID).Set(val)
						}
					}
				}

				// --- Uptime ---
				internal.NodeUptime.WithLabelValues(wsID, nodeID).Set(time.Since(mw.startTime).Seconds())

			case <-mw.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StartPeerStatusMetrics collects WireGuard peer connection status and cumulative traffic.
func (mw *MetricWorker) StartPeerStatusMetrics(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	wsID := mw.identity.WorkspaceID
	nodeID := mw.identity.NodeID
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				metrics, err := mw.peerStatusCollector.Collect()
				if err != nil {
					continue
				}

				// Reset before each cycle so offline peers don't leave stale series.
				internal.PeerStatus.Reset()

				for _, m := range metrics {
					val, ok := m.Value().(float64)
					if !ok {
						continue
					}
					labels := m.Labels()

					switch m.Name() {
					case "peer_status":
						internal.PeerStatus.WithLabelValues(
							wsID,
							nodeID,
							labels["peer_id"],
							labels["endpoint"],
							labels["alias"],
						).Set(val)

					case "peer_receive_bytes":
						mw.addTrafficDelta(wsID, nodeID, labels["peer_id"], "rx", val)

					case "peer_transmit_bytes":
						mw.addTrafficDelta(wsID, nodeID, labels["peer_id"], "tx", val)
					}
				}

			case <-mw.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// addTrafficDelta adds the byte increment since last sample to the PeerTrafficBytes counter.
// Negative deltas (counter reset after interface restart) are silently ignored.
func (mw *MetricWorker) addTrafficDelta(wsID, nodeID, peerID, direction string, current float64) {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	var prevMap map[string]float64
	if direction == "rx" {
		prevMap = mw.prevRxBytes
	} else {
		prevMap = mw.prevTxBytes
	}

	prev, exists := prevMap[peerID]
	prevMap[peerID] = current
	if !exists {
		return // first sample; nothing to add yet
	}
	delta := current - prev
	if delta <= 0 {
		return // no change or counter reset
	}
	internal.PeerTrafficBytes.WithLabelValues(wsID, nodeID, peerID, direction).Add(delta)
}
