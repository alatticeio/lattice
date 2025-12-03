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

package collector

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

type CPUCollector struct {
	// 可配置参数
}

func (c *CPUCollector) Name() string {
	return "cpu"
}

func (c *CPUCollector) Collect() ([]Metric, error) {
	metrics := make([]Metric, 0)

	// 获取CPU使用率
	percentages, err := cpu.Percent(time.Second, true)
	if err != nil {
		return nil, err
	}

	totalPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}

	// 总体CPU指标
	metrics = append(metrics, NewSimpleMetric(
		"cpu_usage_total",
		totalPercent[0],
		map[string]string{"type": "total"},
		time.Now(),
		"total CPU usage",
	))

	// 各核心CPU指标
	for i, p := range percentages {
		metrics = append(metrics, NewSimpleMetric(
			"cpu_usage_core",
			p,
			map[string]string{"core": fmt.Sprintf("%d", i)},
			time.Now(),
			"every cpu usage",
		))
	}

	return metrics, nil
}
