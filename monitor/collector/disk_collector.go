package collector

import (
	"github.com/shirou/gopsutil/v4/disk"
	"log"
	"time"
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
