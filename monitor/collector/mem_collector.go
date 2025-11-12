// Copyright 2025 Wireflow.io, Inc.
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
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

type MemoryCollector struct{}

func (c *MemoryCollector) Name() string {
	return "memory"
}

func (c *MemoryCollector) Collect() ([]Metric, error) {
	metrics := make([]Metric, 0)

	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	now := time.Now()

	metrics = append(metrics, NewSimpleMetric(
		"memory_total",
		float64(memStats.Total),
		nil,
		now,
		"memory total",
	))

	metrics = append(metrics, NewSimpleMetric(
		"memory_used",
		float64(memStats.Used),
		nil,
		now,
		"memory used",
	))

	metrics = append(metrics, NewSimpleMetric(
		"memory_free",
		float64(memStats.Free),
		nil,
		now,
		"memory free",
	))

	metrics = append(metrics, NewSimpleMetric(
		"memory_used_percent",
		memStats.UsedPercent,
		nil,
		now,
		"memory used_percent",
	))

	return metrics, nil
}
