package collector

import (
	"github.com/shirou/gopsutil/v4/mem"
	"time"
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
