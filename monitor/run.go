package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MonitorRunner struct {
	log   *log.Logger
	peers *infra.PeerManager
}

func NewMonitorRunner(peers *infra.PeerManager) *MonitorRunner {
	return &MonitorRunner{
		log:   log.GetLogger("monitor"),
		peers: peers,
	}
}

func (r *MonitorRunner) Run(ctx context.Context) error {
	// 1. 初始化监控服务器 (暴露 /metrics)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:    ":9586",
		Handler: mux,
	}

	// 2. 启动后台采集协程
	worker := NewMetricWorker()

	go func() {
		<-ctx.Done()
		fmt.Printf("Metrics shutting down")
		// 给 Server 5 秒钟处理最后的请求
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("Metrics Server 关闭失败: %v\n", err)
		}

	}()

	// 链路探测：每 15 秒一次
	worker.StartLinkProbing(ctx, 15*time.Second)

	// 系统指标：每 1 分钟一次
	worker.StartSystemMetrics(ctx, 10*time.Second)

	worker.StartPeerStatusMetrics(ctx, 15*time.Second)

	// 3. 主线程 hold 住
	fmt.Printf("Metrics Server 启动在 %s\n", server.Addr)
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		// 这是正常关闭，不当作错误返回
		return nil
	}
	return err

}
