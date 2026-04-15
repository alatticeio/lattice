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
	"fmt"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"

	"wireflow/internal/infra"
)

// WireGuardScraper collects per-peer WireGuard counters and computes aggregated
// node-level and workspace-level traffic statistics.
//
// Emitted metrics:
//
//	wireflow_peer_status                 {peer_id, network_id, remote_peer_id, remote_peer_name, endpoint}
//	wireflow_peer_last_handshake_seconds {peer_id, network_id, remote_peer_id}
//	wireflow_peer_traffic_bytes_total    {peer_id, network_id, remote_peer_id, remote_peer_name, direction}
//	wireflow_node_traffic_bytes_total    {peer_id, network_id, direction}
//	wireflow_peering_traffic_bytes_total {peer_id, local_network_id, remote_network_id, direction}
//
// workspace-level rollup in VM/PromQL:
//
//	sum by (network_id, direction) (wireflow_node_traffic_bytes_total)
type WireGuardScraper struct {
	peers *infra.PeerManager
	wgctl *wgctrl.Client
}

// NewWireGuardScraper creates a WireGuardScraper.
// The caller should not call Close() on the returned scraper; the Collector
// engine manages lifecycle via context cancellation.
func NewWireGuardScraper(peers *infra.PeerManager) (*WireGuardScraper, error) {
	cl, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("wgctrl.New: %w", err)
	}
	return &WireGuardScraper{peers: peers, wgctl: cl}, nil
}

func (s *WireGuardScraper) Name() string { return "wireguard" }

// Scrape implements Scraper.
func (s *WireGuardScraper) Scrape(_ context.Context, id Identity, nowMs int64) ([]Sample, error) {
	dev, err := s.wgctl.Device(id.Interface)
	if err != nil {
		return nil, fmt.Errorf("wgctrl Device(%s): %w", id.Interface, err)
	}

	now := time.Now()
	nodeBase := Labels{"peer_id": id.PeerID, "network_id": id.NetworkID}

	var (
		out     []Sample
		totalRx float64
		totalTx float64
		// peering accumulator: remote_network_id → (rx, tx)
		peeringRx = make(map[string]float64)
		peeringTx = make(map[string]float64)
	)

	for _, wgp := range dev.Peers {
		// ── Resolve business metadata from PeerManager ────────────────────
		remotePeerID := wgp.PublicKey.String()
		var remoteName, remoteNetwork string

		pid := infra.FromKey(wgp.PublicKey)
		if meta := s.peers.GetByPeerID(pid); meta != nil {
			remotePeerID = meta.AppID
			remoteName = meta.Name
			remoteNetwork = meta.NetworkId
		} else {
			// Unknown peer: truncate pubkey as fallback identifier.
			if len(remotePeerID) > 16 {
				remotePeerID = remotePeerID[:16]
			}
			remoteName = remotePeerID
		}
		if remoteNetwork == "" {
			remoteNetwork = id.NetworkID // assume same workspace if unknown
		}

		// ── Derived values ────────────────────────────────────────────────
		rxBytes := float64(wgp.ReceiveBytes)
		txBytes := float64(wgp.TransmitBytes)

		var lastHS float64
		if !wgp.LastHandshakeTime.IsZero() {
			lastHS = float64(wgp.LastHandshakeTime.Unix())
		}

		status := 0.0
		if !wgp.LastHandshakeTime.IsZero() && now.Sub(wgp.LastHandshakeTime) < 3*time.Minute {
			status = 1.0
		}

		endpoint := ""
		if wgp.Endpoint != nil {
			endpoint = wgp.Endpoint.String()
		}

		// ── Per-peer samples ──────────────────────────────────────────────
		statusLbls := mergeLabels(nodeBase, Labels{
			"remote_peer_id":   remotePeerID,
			"remote_peer_name": remoteName,
			"endpoint":         endpoint,
		})
		out = append(out, NewSample("wireflow_peer_status", statusLbls, status, nowMs))

		hsLbls := mergeLabels(nodeBase, Labels{"remote_peer_id": remotePeerID})
		out = append(out, NewSample("wireflow_peer_last_handshake_seconds", hsLbls, lastHS, nowMs))

		trafficLbls := mergeLabels(nodeBase, Labels{
			"remote_peer_id":   remotePeerID,
			"remote_peer_name": remoteName,
		})
		out = append(out,
			NewSample("wireflow_peer_traffic_bytes_total",
				mergeLabels(trafficLbls, Labels{"direction": "rx"}), rxBytes, nowMs),
			NewSample("wireflow_peer_traffic_bytes_total",
				mergeLabels(trafficLbls, Labels{"direction": "tx"}), txBytes, nowMs),
		)

		// ── Accumulate for aggregate metrics ─────────────────────────────
		totalRx += rxBytes
		totalTx += txBytes

		if remoteNetwork != id.NetworkID {
			peeringRx[remoteNetwork] += rxBytes
			peeringTx[remoteNetwork] += txBytes
		}
	}

	// ── Node-level aggregate traffic ──────────────────────────────────────
	// VM query for workspace-level: sum by (network_id, direction) (wireflow_node_traffic_bytes_total)
	out = append(out,
		NewSample("wireflow_node_traffic_bytes_total",
			mergeLabels(nodeBase, Labels{"direction": "rx"}), totalRx, nowMs),
		NewSample("wireflow_node_traffic_bytes_total",
			mergeLabels(nodeBase, Labels{"direction": "tx"}), totalTx, nowMs),
	)

	// ── Cross-workspace peering traffic ───────────────────────────────────
	for remoteNID, rx := range peeringRx {
		peeringLbls := Labels{
			"peer_id":           id.PeerID,
			"local_network_id":  id.NetworkID,
			"remote_network_id": remoteNID,
		}
		out = append(out,
			NewSample("wireflow_peering_traffic_bytes_total",
				mergeLabels(peeringLbls, Labels{"direction": "rx"}), rx, nowMs),
			NewSample("wireflow_peering_traffic_bytes_total",
				mergeLabels(peeringLbls, Labels{"direction": "tx"}), peeringTx[remoteNID], nowMs),
		)
	}

	return out, nil
}
