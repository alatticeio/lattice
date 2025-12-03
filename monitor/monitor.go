// Copyright 2025 wireflowio.com, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monitor

import (
	"time"
	"wireflow/monitor/collector"
	"wireflow/pkg/log"
)

// NodeMonitor 节点监控器
type NodeMonitor struct {
	collectors []collector.MetricCollector
	interval   time.Duration
	storage    collector.Storage
	alerter    collector.Alerter
	stopChan   chan struct{}
	logger     *log.Logger
}

func NewNodeMonitor(interval time.Duration, storage collector.Storage, alerter collector.Alerter) *NodeMonitor {
	return &NodeMonitor{
		collectors: make([]collector.MetricCollector, 0),
		interval:   interval,
		storage:    storage,
		alerter:    alerter,
		stopChan:   make(chan struct{}),
		logger:     log.NewLogger(log.Loglevel, "monitor"),
	}
}

func (m *NodeMonitor) AddCollector(collector collector.MetricCollector) {
	m.collectors = append(m.collectors, collector)
}

func (m *NodeMonitor) Start() error {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopChan:
				return
			case <-ticker.C:
				allMetrics := make([]collector.Metric, 0)

				// 从所有收集器获取指标
				for _, collector := range m.collectors {
					metrics, err := collector.Collect()
					if err != nil {
						m.logger.Errorf("Error collecting metrics from %s: %v", collector.Name(), err)
						continue
					}
					allMetrics = append(allMetrics, metrics...)
				}

				// 存储指标数据
				if err := m.storage.Store(allMetrics); err != nil {
					m.logger.Errorf("Error storing metrics: %v", err)
				}

				// 告警检查
				//alerts, err := m.alerter.Evaluate(allMetrics)
				//if err != nil {
				//	log.Printf("Error evaluating alerts: %v", err)
				//} else if len(alerts) > 0 {
				//	if err := m.alerter.Send(alerts); err != nil {
				//		log.Printf("Error sending alerts: %v", err)
				//	}
				//}

				m.logger.Verbosef("Storing %d metrics", len(allMetrics))
			}
		}
	}()
	return nil
}

func (m *NodeMonitor) Stop() {
	close(m.stopChan)
}
