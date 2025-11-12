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

package monitor

import (
	"fmt"
	"log"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

type SystemStats struct {
	CPUUsage      []float64
	CPUTotalUsage float64
	DiskUsage
	MemoryUsage *mem.VirtualMemoryStat
	Timestamp   time.Time
}

type DiskUsage struct {
	UsedPercent float64
	Total       float64
	Used        float64
	Free        float64
}

type CPUMonitor struct {
	interval  time.Duration
	statsChan chan SystemStats
	stopChan  chan struct{}
}

func NewCPUMonitor(internal time.Duration) *CPUMonitor {
	return &CPUMonitor{
		interval:  internal,
		statsChan: make(chan SystemStats, 100),
		stopChan:  make(chan struct{}),
	}
}

func (m *CPUMonitor) Start() error {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopChan:
				return
			case <-ticker.C:
				stats, err := m.collectStats()
				if err != nil {
					log.Printf("Error collecting system stats: %v", err)
					continue
				}
				m.statsChan <- stats
			}
		}
	}()
	return nil
}

func (m *CPUMonitor) collectStats() (SystemStats, error) {
	// 获取 CPU 使用率
	cpuPercentages, err := cpu.Percent(time.Second, true)
	if err != nil {
		return SystemStats{}, fmt.Errorf("failed to get CPU usage: %v", err)
	}

	totalPercentage, err := cpu.Percent(time.Second, false)

	// 获取内存使用情况
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return SystemStats{}, fmt.Errorf("failed to get memory stats: %v", err)
	}

	// 获取磁盘使用情况
	diskStats, err := disk.Usage("/")

	diskUsage := DiskUsage{
		UsedPercent: diskStats.UsedPercent,
		Total:       float64(diskStats.Total),
		Used:        float64(diskStats.Used),
		Free:        float64(diskStats.Free),
	}

	return SystemStats{
		CPUUsage:      cpuPercentages,
		CPUTotalUsage: totalPercentage[0],
		DiskUsage:     diskUsage,
		MemoryUsage:   memStats,
		Timestamp:     time.Now(),
	}, nil
}

func (m *CPUMonitor) Stop() {
	close(m.stopChan)
}

func (m *CPUMonitor) GetStats() <-chan SystemStats {
	return m.statsChan
}

// CPU 信息获取
func GetCPUInfo() error {
	// 获取 CPU 信息
	cpuInfo, err := cpu.Info()
	if err != nil {
		return fmt.Errorf("failed to get CPU info: %v", err)
	}

	// 打印每个 CPU 的信息
	for _, cpu := range cpuInfo {
		fmt.Printf("CPU ID: %d\n", cpu.CPU)
		fmt.Printf("VendorID: %s\n", cpu.VendorID)
		fmt.Printf("Family: %s\n", cpu.Family)
		fmt.Printf("Model: %s\n", cpu.Model)
		fmt.Printf("Cores: %d\n", cpu.Cores)
		fmt.Printf("MHz: %.2f\n", cpu.Mhz)
	}

	return nil
}

// CPU 负载监控
func MonitorCPULoad() error {
	// 获取 CPU 数量
	counts, err := cpu.Counts(true)
	if err != nil {
		return fmt.Errorf("failed to get CPU counts: %v", err)
	}

	fmt.Printf("CPU 核心数: %d\n", counts)

	// 监控 CPU 使用率
	for {
		percentages, err := cpu.Percent(time.Second, true)
		if err != nil {
			return fmt.Errorf("failed to get CPU percentages: %v", err)
		}

		for i, percentage := range percentages {
			fmt.Printf("CPU %d 使用率: %.2f%%\n", i, percentage)
		}

		time.Sleep(time.Second)
	}
}

func main() {
	// 创建监控器
	monitor := NewCPUMonitor(time.Second * 2)

	// 启动监控
	if err := monitor.Start(); err != nil {
		log.Fatal(err)
	}
	defer monitor.Stop()

	// 打印 CPU 信息
	if err := GetCPUInfo(); err != nil {
		log.Printf("Error getting CPU info: %v", err)
	}

	// 处理监控数据
	go func() {
		for stats := range monitor.GetStats() {
			fmt.Printf("\n时间: %s\n", stats.Timestamp.Format("2006-01-02 15:04:05"))

			// 打印 CPU 使用率
			for i, usage := range stats.CPUUsage {
				fmt.Printf("CPU %d 使用率: %.2f%%\n", i, usage)
				fmt.Printf("CPU 总体使用率: %.2f%%\n", stats.CPUTotalUsage)
			}

			// 打印内存使用情况
			fmt.Printf("内存使用率: %.2f%%\n", stats.MemoryUsage.UsedPercent)
			fmt.Printf("总内存: %v GB\n", float64(stats.MemoryUsage.Total)/(1024*1024*1024))
			fmt.Printf("已用内存: %v GB\n", float64(stats.MemoryUsage.Used)/(1024*1024*1024))
			fmt.Printf("可用内存: %v GB\n", float64(stats.MemoryUsage.Available)/(1024*1024*1024))
		}
	}()

	// 运行一段时间后退出
	time.Sleep(time.Second * 20)
}
