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
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

type CPUCollector struct {
	// 可配置参数
}

func NewCPUCollector() *CPUCollector {
	return &CPUCollector{}
}

func (c *CPUCollector) Name() string {
	return "cpu"
}

func (c *CPUCollector) Collect() ([]Metric, error) {
	metrics := make([]Metric, 0)
	now := time.Now()

	// 只阻塞一次，获取分核数据
	percentages, err := cpu.Percent(time.Second, true)
	if err != nil {
		return nil, err
	}

	var totalSum float64
	for i, p := range percentages {
		// 各核心指标
		metrics = append(metrics, NewSimpleMetric(
			"cpu_usage_core",
			p,
			map[string]string{"core": fmt.Sprintf("%d", i)},
			now, // 统一使用同一个时间戳
			"every cpu usage",
		))
		totalSum += p
	}

	// 通过分核数据算平均值，避免再次调用 cpu.Percent 阻塞 1 秒
	avgPercent := totalSum / float64(len(percentages))

	// 总体CPU指标
	metrics = append(metrics, NewSimpleMetric(
		"cpu_usage_total",
		avgPercent,
		map[string]string{"type": "total"},
		now,
		"total CPU usage",
	))

	return metrics, nil
}
