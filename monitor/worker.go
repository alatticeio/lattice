package monitor

import (
	"context"
	"fmt"
	"time"
	"wireflow/monitor/collector"
	exporter "wireflow/monitor/wireflow-exporter"
)

// MetricWorker 定义采集管理结构
type MetricWorker struct {
	stopChan            chan struct{}
	cpuCollector        collector.MetricCollector
	peerStatusCollector collector.MetricCollector
}

func NewMetricWorker() *MetricWorker {
	return &MetricWorker{
		stopChan:            make(chan struct{}),
		cpuCollector:        collector.NewCPUCollector(),
		peerStatusCollector: collector.NewPeerStatusCollector(),
	}
}

// Start 背景运行：延迟与链路探测
func (mw *MetricWorker) StartLinkProbing(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// 1. 获取最新的 Peer 列表 (从你的 core 模块获取)
				// targets := core.GetActivePeers()
				// 2. 执行并发探测
				// RunCycle(targets)
				fmt.Println("start link probing")
			case <-mw.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Start 系统指标采集（CPU/MEM）
func (mw *MetricWorker) StartSystemMetrics(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				metrics, err := mw.cpuCollector.Collect()
				if err != nil {
					// 记录日志，不要让程序崩掉
					continue
				}

				for _, m := range metrics {
					// 获取原始值，并断言为 float64
					// 注意：gopsutil 返回的百分比通常已经是 float64 了
					val, ok := m.Value().(float64)
					if !ok {
						// 如果断言失败，记录日志或跳过，防止程序崩溃
						// log.Printf("metric %s has invalid value type", m.Name())
						continue
					}

					switch m.Name() {
					case "cpu_usage_total":
						exporter.NodeCpuUsage.Set(val)

					case "cpu_usage_core":
						coreID := m.Labels()["core"]
						exporter.NodeCoreUsage.WithLabelValues(coreID).Set(val)
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

func (mw *MetricWorker) StartPeerStatusMetrics(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// 1. 调用 Peer 状态采集器（注意这里要换成你对应的 Collector 实例）
				metrics, err := mw.peerStatusCollector.Collect()
				if err != nil {
					// log.Printf("failed to collect peer status: %v", err)
					continue
				}

				// 2. 核心操作：在写入这一批次数据前，先重置 Gauge，防止已下线的节点残留
				exporter.PeerStatus.Reset()
				// 如果你顺便采集了流量，也在这里重置对应的 Gauge/Counter
				// exporter.PeerBytesTransmit.Reset()
				// exporter.PeerBytesReceive.Reset()

				// 3. 遍历并设置指标
				for _, m := range metrics {
					val, ok := m.Value().(float64)
					if !ok {
						continue
					}

					labels := m.Labels()

					// 根据指标名称分发数据
					switch m.Name() {
					case "peer_status":
						// 确保这里的 Label 顺序与你定义 PeerStatus 时一致
						// 建议：peer_id, ip, alias
						exporter.PeerStatus.WithLabelValues(
							labels["peer_id"],
							labels["ip"],
							labels["alias"],
						).Set(val)

						//TODO should Add traffic datas
						//case "peer_receive_bytes":
						//	exporter.PeerBytesReceive.WithLabelValues(
						//		labels["peer_id"],
						//		labels["ip"],
						//		labels["alias"],
						//	).Set(val)
						//
						//case "peer_transmit_bytes":
						//	exporter.PeerBytesTransmit.WithLabelValues(
						//		labels["peer_id"],
						//		labels["ip"],
						//		labels["alias"],
						//	).Set(val)
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
