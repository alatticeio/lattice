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
	"log"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

type DiskCollector struct {
	paths []string
}

func NewDiskCollector(paths []string) *DiskCollector {
	if len(paths) == 0 {
		paths = []string{"/"}
	}
	return &DiskCollector{paths: paths}
}

func (c *DiskCollector) Name() string {
	return "disk"
}

func (c *DiskCollector) Collect() ([]Metric, error) {
	metrics := make([]Metric, 0)
	now := time.Now()

	for _, path := range c.paths {
		diskStats, err := disk.Usage(path)
		if err != nil {
			log.Printf("Error getting disk stats for %s: %v", path, err)
			continue
		}

		metrics = append(metrics, NewSimpleMetric(
			"disk_total",
			float64(diskStats.Total),
			map[string]string{"path": path},
			now,
			"disk total usage",
		))

		metrics = append(metrics, NewSimpleMetric(
			"disk_used",
			float64(diskStats.Used),
			map[string]string{"path": path},
			now,
			"disk used usage",
		))

		metrics = append(metrics, NewSimpleMetric(
			"disk_free",
			float64(diskStats.Free),
			map[string]string{"path": path},
			now,
			"disk free usage",
		))

		metrics = append(metrics, NewSimpleMetric(
			"disk_used_percent",
			diskStats.UsedPercent,
			map[string]string{"path": path},
			now,
			"disk used percent",
		))
	}

	return metrics, nil
}
