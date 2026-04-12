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

package telemetry

import (
	"context"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
	gopsutilmem "github.com/shirou/gopsutil/v4/mem"
)

// SystemScraper collects host-level system metrics.
//
// Emitted metrics:
//
//	wireflow_node_cpu_usage_percent {peer_id, network_id}
//	wireflow_node_memory_bytes      {peer_id, network_id}
//	wireflow_node_goroutines        {peer_id, network_id}
type SystemScraper struct{}

// NewSystemScraper creates a SystemScraper.
func NewSystemScraper() *SystemScraper { return &SystemScraper{} }

func (s *SystemScraper) Name() string { return "system" }

// Scrape implements Scraper.
func (s *SystemScraper) Scrape(_ context.Context, id Identity, nowMs int64) ([]Sample, error) {
	base := Labels{"peer_id": id.PeerID, "network_id": id.NetworkID}
	out := make([]Sample, 0, 3)

	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		out = append(out, NewSample("wireflow_node_cpu_usage_percent", base, pcts[0], nowMs))
	}

	if vm, err := gopsutilmem.VirtualMemory(); err == nil {
		out = append(out, NewSample("wireflow_node_memory_bytes", base, float64(vm.Used), nowMs))
	}

	out = append(out, NewSample("wireflow_node_goroutines", base, float64(runtime.NumGoroutine()), nowMs))

	return out, nil
}
