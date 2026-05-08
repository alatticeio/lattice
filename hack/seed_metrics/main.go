// hack/seed_metrics/main.go
//
// Development utility: push mock metrics to VictoriaMetrics for local Dashboard testing.
//
// Usage:
//
//	go run ./hack/seed_metrics \
//	  --vm-url http://101.36.119.12:8428 \
//	  --network-id wf-your-workspace-namespace \
//	  --nodes 5 \
//	  --interval 30s
//
// network-id corresponds to the t_workspace.namespace field in the database.
// Query it with: SELECT namespace FROM t_workspace;

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var (
	vmURL     = flag.String("vm-url", "http://101.36.119.12:8428", "VictoriaMetrics URL")
	networkID = flag.String("network-id", "", "workspace.Namespace (= network_id label), required")
	nodeCount = flag.Int("nodes", 4, "number of simulated nodes")
	interval  = flag.Duration("interval", 30*time.Second, "push interval")
)

type nodeState struct {
	cpu     float64
	memMB   float64
	txBytes float64
	rxBytes float64
	latency float64
	loss    float64
	uptime  float64
}

func main() {
	flag.Parse()
	if *networkID == "" {
		log.Fatal("--network-id is required. Query the DB first: SELECT namespace FROM t_workspace;")
	}

	log.Printf("▶ seed_metrics starting")
	log.Printf("  vm-url:     %s", *vmURL)
	log.Printf("  network-id: %s", *networkID)
	log.Printf("  nodes:      %d", *nodeCount)
	log.Printf("  interval:   %s", *interval)
	log.Println()

	// Generate a stable list of node IDs
	nodes := make([]string, *nodeCount)
	for i := range nodes {
		nodes[i] = fmt.Sprintf("node-%02d", i+1)
	}

	// Each node maintains its own random state to simulate real fluctuations
	states := make([]nodeState, *nodeCount)
	for i := range states {
		states[i].cpu = 20 + rand.Float64()*40
		states[i].memMB = 256 + rand.Float64()*512
		states[i].txBytes = rand.Float64() * 1e9
		states[i].rxBytes = rand.Float64() * 5e8
		states[i].latency = 5 + rand.Float64()*30
		states[i].loss = rand.Float64() * 0.5
		states[i].uptime = float64(i*300) + rand.Float64()*3600
	}

	tick := time.NewTicker(*interval)
	defer tick.Stop()

	// Push once immediately
	push(nodes, states, *vmURL, *networkID)

	for range tick.C {
		// Update state (simulate real fluctuations)
		for i := range states {
			s := &states[i]
			s.cpu = clamp(s.cpu+jitter(10), 2, 98)
			s.memMB = clamp(s.memMB+jitter(50), 64, 1024)
			// Traffic is monotonically increasing
			txDelta := 1e6 + rand.Float64()*50e6
			rxDelta := 5e5 + rand.Float64()*25e6
			s.txBytes += txDelta
			s.rxBytes += rxDelta
			s.latency = clamp(s.latency+jitter(5), 1, 200)
			s.loss = clamp(s.loss+jitter(0.3), 0, 5)
			s.uptime += interval.Seconds()
		}
		push(nodes, states, *vmURL, *networkID)
	}
}

func push(nodes []string, states []nodeState, vmURL, networkID string) {
	var sb strings.Builder
	now := time.Now().UnixMilli()

	for i, node := range nodes {
		s := states[i]
		base := fmt.Sprintf(`peer_id="%s",network_id="%s"`, node, networkID)

		// ── system ────────────────────────────────────────────────
		writeLine(&sb, "lattice_node_cpu_usage_percent", base, s.cpu, now)
		writeLine(&sb, "lattice_node_memory_bytes", base, s.memMB*1e6, now)
		writeLine(&sb, "lattice_node_goroutines", base, 80+rand.Float64()*40, now)
		writeLine(&sb, "lattice_node_uptime_seconds", base, s.uptime, now)

		// ── wireguard traffic ─────────────────────────────────────
		txBase := fmt.Sprintf(`peer_id="%s",network_id="%s",direction="tx"`, node, networkID)
		rxBase := fmt.Sprintf(`peer_id="%s",network_id="%s",direction="rx"`, node, networkID)
		writeLine(&sb, "lattice_node_traffic_bytes_total", txBase, s.txBytes, now)
		writeLine(&sb, "lattice_node_traffic_bytes_total", rxBase, s.rxBytes, now)

		// ── peer connections (mesh: each node peers with every other) ──
		for j, remote := range nodes {
			if i == j {
				continue
			}
			latency := s.latency + jitter(3)
			peerBase := fmt.Sprintf(`peer_id="%s",network_id="%s",remote_peer_id="%s",remote_peer_name="%s"`,
				node, networkID, remote, remote)
			peerBaseEP := fmt.Sprintf(`peer_id="%s",network_id="%s",remote_peer_id="%s",remote_peer_name="%s",endpoint="10.0.0.%d:51820"`,
				node, networkID, remote, remote, j+1)

			writeLine(&sb, "lattice_peer_status", peerBaseEP, 1.0, now)

			hs := float64(time.Now().Add(-time.Duration(30+rand.Intn(90)) * time.Second).Unix())
			writeLine(&sb, "lattice_peer_last_handshake_seconds",
				fmt.Sprintf(`peer_id="%s",network_id="%s",remote_peer_id="%s"`, node, networkID, remote),
				hs, now)

			// per-peer traffic
			peerTxBase := peerBase + `,direction="tx"`
			peerRxBase := peerBase + `,direction="rx"`
			writeLine(&sb, "lattice_peer_traffic_bytes_total", peerTxBase, s.txBytes/float64(len(nodes)-1), now)
			writeLine(&sb, "lattice_peer_traffic_bytes_total", peerRxBase, s.rxBytes/float64(len(nodes)-1), now)

			// ICMP
			latBase := fmt.Sprintf(`peer_id="%s",network_id="%s",remote_peer_id="%s",remote_peer_name="%s",remote_peer_ip="10.0.0.%d"`,
				node, networkID, remote, remote, j+1)
			lossBase := fmt.Sprintf(`peer_id="%s",network_id="%s",remote_peer_id="%s"`,
				node, networkID, remote)
			writeLine(&sb, "lattice_peer_latency_ms", latBase, clamp(latency, 1, 200), now)
			writeLine(&sb, "lattice_peer_packet_loss_percent", lossBase, clamp(s.loss, 0, 5), now)
		}
	}

	body := sb.String()
	url := vmURL + "/api/v1/import/prometheus"
	resp, err := http.Post(url, "text/plain", bytes.NewBufferString(body)) //nolint:noctx
	if err != nil {
		log.Printf("✗ push failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		log.Printf("✗ VM returned HTTP %d", resp.StatusCode)
		return
	}
	log.Printf("✓ pushed %d nodes × %d metrics  [%s]",
		len(nodes), strings.Count(body, "\n"), time.Now().Format("15:04:05"))
}

func writeLine(sb *strings.Builder, name, labels string, value float64, tsMs int64) {
	fmt.Fprintf(sb, "%s{%s} %g %d\n", name, labels, value, tsMs)
}

// jitter returns a random delta in [-half, +half].
func jitter(half float64) float64 {
	return (rand.Float64()*2 - 1) * half
}

func clamp(v, min, max float64) float64 {
	return math.Max(min, math.Min(max, v))
}
