// Copyright 2025 The Wireflow Authors, Inc.
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

package collector

import (
	"fmt"
	"sync"
	"time"
	"wireflow/internal/infra"

	"github.com/shirou/gopsutil/v4/cpu"
)

type CPUCollector struct {
	peerManager *infra.PeerManager // 保持和你项目架构一致

	// 缓存数据
	mu           sync.RWMutex
	totalPercent float64
	corePercents []float64
	lastUpdate   time.Time
}

func (c *CPUCollector) Name() string {
	return "cpu_collector"
}

func NewCPUCollector() MetricCollector {
	c := &CPUCollector{
		peerManager: infra.NewPeerManager(),
	}
	// 启动后台异步采集协程
	go c.runSnapshotter()
	return c
}

func (c *CPUCollector) runSnapshotter() {
	// 建议采集频率略高于或等于监控抓取频率
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 只需要阻塞一次，true 表示返回各核，其平均值即为总计
		percents, err := cpu.Percent(time.Second, true)
		if err != nil || len(percents) == 0 {
			continue
		}

		// 计算总百分比
		var total float64
		for _, p := range percents {
			total += p
		}
		avgTotal := total / float64(len(percents))

		// 更新缓存
		c.mu.Lock()
		c.totalPercent = avgTotal
		c.corePercents = percents
		c.lastUpdate = time.Now()
		c.mu.Unlock()
	}
}

func (c *CPUCollector) Collect() ([]Metric, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := make([]Metric, 0)
	now := time.Now()

	// 1. 总体 CPU 指标（直接读内存，耗时为纳秒级）
	metrics = append(metrics, NewSimpleMetric(
		"cpu_usage_total",
		c.totalPercent,
		map[string]string{"type": "total"},
		now,
		"total CPU usage",
	))

	// 2. 各核心 CPU 指标
	for i, p := range c.corePercents {
		metrics = append(metrics, NewSimpleMetric(
			"cpu_usage_core",
			p,
			map[string]string{"core": fmt.Sprintf("%d", i)},
			now,
			"per core cpu usage",
		))
	}

	return metrics, nil
}
