package wireflow_exporter

import (
	"sync"
	"time"
	"wireflow/internal"

	probing "github.com/prometheus-community/pro-bing"
)

// TargetPeer is the probe target passed in from the core networking layer.
type TargetPeer struct {
	ID   string
	Name string
	IP   string
}

func NewTargetPeer(id string, name string, ip string) *TargetPeer {
	return &TargetPeer{
		ID:   id,
		Name: name,
		IP:   ip,
	}
}

// RunCycle probes each target peer concurrently and records latency / packet-loss metrics.
// workspaceID and nodeID are the local node's identity labels.
func RunCycle(workspaceID, nodeID string, targets []TargetPeer) {
	var wg sync.WaitGroup

	for _, t := range targets {
		wg.Add(1)
		go func(target TargetPeer) {
			defer wg.Done()

			pinger, err := probing.NewPinger(target.IP)
			if err != nil {
				internal.PeerLoss.WithLabelValues(workspaceID, nodeID, target.ID).Set(100)
				return
			}

			pinger.Count = 3
			pinger.Timeout = 2 * time.Second
			pinger.SetPrivileged(false)

			if err = pinger.Run(); err != nil {
				internal.PeerLoss.WithLabelValues(workspaceID, nodeID, target.ID).Set(100)
				return
			}

			stats := pinger.Statistics()
			internal.PeerLatency.WithLabelValues(workspaceID, nodeID, target.ID, target.Name, target.IP).Set(float64(stats.AvgRtt.Milliseconds()))
			internal.PeerLoss.WithLabelValues(workspaceID, nodeID, target.ID).Set(stats.PacketLoss)
		}(t)
	}
	wg.Wait()
}
