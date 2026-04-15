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

//go:build pro

package telemetry

import (
	"context"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"

	"wireflow/internal/infra"
)

// ICMPScraper probes each peer VIP with ICMP echo requests and records latency
// and packet-loss. Probes run concurrently so total wall time equals PingTimeout,
// not PingTimeout × peer count.
//
// Privilege: uses unprivileged UDP ICMP (SetPrivileged(false)).
// On Linux this requires: sysctl -w net.ipv4.ping_group_range="0 2147483647"
// Errors from individual peers are non-fatal; that peer gets packetLoss = 100%.
//
// Emitted metrics:
//
//	wireflow_peer_latency_ms          {peer_id, network_id, remote_peer_id, remote_peer_name, remote_peer_ip}
//	wireflow_peer_packet_loss_percent {peer_id, network_id, remote_peer_id}
type ICMPScraper struct {
	peers   *infra.PeerManager
	count   int
	timeout time.Duration
}

// NewICMPScraper creates an ICMPScraper.
//
//   - count:   number of ICMP echo requests per peer per cycle (default 3)
//   - timeout: per-probe timeout (default 2 s)
func NewICMPScraper(peers *infra.PeerManager, count int, timeout time.Duration) *ICMPScraper {
	if count <= 0 {
		count = 3
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &ICMPScraper{peers: peers, count: count, timeout: timeout}
}

func (s *ICMPScraper) Name() string { return "icmp" }

// Scrape implements Scraper.
func (s *ICMPScraper) Scrape(ctx context.Context, id Identity, nowMs int64) ([]Sample, error) {
	// Build probe target list from PeerManager.
	allPeers := s.peers.GetAll()
	type target struct {
		appID   string
		name    string
		ip      string
		network string
	}
	targets := make([]target, 0, len(allPeers))
	for _, p := range allPeers {
		if p.Address == nil || *p.Address == "" || *p.Address == "0.0.0.0" {
			continue
		}
		targets = append(targets, target{
			appID:   p.AppID,
			name:    p.Name,
			ip:      *p.Address,
			network: p.NetworkId,
		})
	}
	if len(targets) == 0 {
		return nil, nil
	}

	type result struct {
		t          target
		latencyMs  float64
		packetLoss float64
	}

	results := make([]result, 0, len(targets))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, t := range targets {
		wg.Add(1)
		go func(t target) {
			defer wg.Done()
			r := result{t: t, packetLoss: 100}

			pinger, err := probing.NewPinger(t.ip)
			if err != nil {
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				return
			}
			pinger.Count = s.count
			pinger.Timeout = s.timeout
			pinger.SetPrivileged(false)

			// Stop pinger if context is cancelled.
			stopCh := make(chan struct{})
			go func() {
				select {
				case <-ctx.Done():
					pinger.Stop()
				case <-stopCh:
				}
			}()

			if err = pinger.Run(); err == nil {
				stats := pinger.Statistics()
				r.latencyMs = float64(stats.AvgRtt.Milliseconds())
				r.packetLoss = stats.PacketLoss
			}
			close(stopCh)

			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(t)
	}
	wg.Wait()

	// Build samples from probe results.
	nodeBase := Labels{"peer_id": id.PeerID, "network_id": id.NetworkID}
	out := make([]Sample, 0, len(results)*2)

	for _, r := range results {
		latencyLbls := mergeLabels(nodeBase, Labels{
			"remote_peer_id":   r.t.appID,
			"remote_peer_name": r.t.name,
			"remote_peer_ip":   r.t.ip,
		})
		lossLbls := mergeLabels(nodeBase, Labels{
			"remote_peer_id": r.t.appID,
		})

		out = append(out,
			NewSample("wireflow_peer_latency_ms", latencyLbls, r.latencyMs, nowMs),
			NewSample("wireflow_peer_packet_loss_percent", lossLbls, r.packetLoss, nowMs),
		)
	}

	return out, nil
}
