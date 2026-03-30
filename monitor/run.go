package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"wireflow/internal/config"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MonitorRunner struct {
	log           *log.Logger
	peers         *infra.PeerManager
	identity      NodeIdentity
	interfaceName string
}

func NewMonitorRunner(peers *infra.PeerManager, interfaceName string) *MonitorRunner {
	nodeID := config.GlobalConfig.AppId
	if nodeID == "" {
		// Fall back to hostname when AppId is not yet configured.
		nodeID = "unknown"
	}
	return &MonitorRunner{
		log:           log.GetLogger("monitor"),
		peers:         peers,
		interfaceName: interfaceName,
		identity: NodeIdentity{
			WorkspaceID: "", // populated after the agent receives its network map
			NodeID:      nodeID,
		},
	}
}

func (r *MonitorRunner) Run(ctx context.Context) error {
	// 1. Expose /metrics endpoint
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:    ":9586",
		Handler: mux,
	}

	// 2. Start background collection workers
	worker := NewMetricWorker(r.identity, r.peers, r.interfaceName)

	go func() {
		<-ctx.Done()
		fmt.Printf("Metrics shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("Metrics Server 关闭失败: %v\n", err)
		}
	}()

	// Link probing: every 15 s
	worker.StartLinkProbing(ctx, 15*time.Second)

	// System metrics (CPU / memory / uptime): every 10 s
	worker.StartSystemMetrics(ctx, 10*time.Second)

	// Peer status + traffic: every 15 s
	worker.StartPeerStatusMetrics(ctx, 15*time.Second)

	fmt.Printf("Metrics Server 启动在 %s\n", server.Addr)
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
