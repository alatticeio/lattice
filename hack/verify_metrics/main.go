// hack/verify_metrics/main.go
//
// 验证工具：检查 wireflow 节点的指标是否已正确上报到 VictoriaMetrics。
//
// 用法:
//
//	go run ./hack/verify_metrics \
//	  --vm-url http://your-vm:8428 \
//	  --network-id <workspace namespace>
//
// --watch 模式会每 30s 刷新一次，方便实时观察节点上线过程。
//
// network-id 对应数据库中 t_workspace.namespace 字段:
//
//	SELECT namespace FROM t_workspace WHERE name = '<your workspace name>';
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	vmURL     = flag.String("vm-url", "http://localhost:8428", "VictoriaMetrics 地址")
	networkID = flag.String("network-id", "", "workspace.namespace 字段值 (= network_id label)")
	watch     = flag.Bool("watch", false, "每 30s 刷新一次")
)

// ── VM query ──────────────────────────────────────────────────────────────────

type vmResult struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []json.RawMessage `json:"value"` // [timestamp, "value"]
		} `json:"result"`
	} `json:"data"`
}

func query(promql string) (*vmResult, error) {
	resp, err := http.PostForm(*vmURL+"/api/v1/query",
		url.Values{"query": {promql}})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r vmResult
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode: %w (body: %s)", err, body[:min(len(body), 200)])
	}
	return &r, nil
}

// scalar returns the first result value as float64, or 0.
func scalar(r *vmResult) float64 {
	if r == nil || len(r.Data.Result) == 0 || len(r.Data.Result[0].Value) < 2 {
		return 0
	}
	raw := strings.Trim(string(r.Data.Result[0].Value[1]), `"`)
	v, _ := strconv.ParseFloat(raw, 64)
	return v
}

func count(r *vmResult) int {
	if r == nil {
		return 0
	}
	return len(r.Data.Result)
}

// ── individual node detail ────────────────────────────────────────────────────

type nodeInfo struct {
	peerID  string
	cpu     float64
	txMbps  float64
	rxMbps  float64
	latency float64
	online  bool
}

func fetchNodes(ns string) []nodeInfo {
	cpuR, _ := query(fmt.Sprintf(
		`last_over_time(wireflow_node_cpu_usage_percent{network_id="%s"}[5m])`, ns))
	txR, _ := query(fmt.Sprintf(
		`irate(wireflow_node_traffic_bytes_total{network_id="%s",direction="tx"}[2m]) * 8 / 1e6`, ns))
	rxR, _ := query(fmt.Sprintf(
		`irate(wireflow_node_traffic_bytes_total{network_id="%s",direction="rx"}[2m]) * 8 / 1e6`, ns))
	latR, _ := query(fmt.Sprintf(
		`avg by (peer_id) (wireflow_peer_latency_ms{network_id="%s"})`, ns))

	// Build by peer_id
	nodes := map[string]*nodeInfo{}
	ensure := func(pid string) *nodeInfo {
		if nodes[pid] == nil {
			nodes[pid] = &nodeInfo{peerID: pid}
		}
		return nodes[pid]
	}

	if cpuR != nil {
		for _, r := range cpuR.Data.Result {
			pid := r.Metric["peer_id"]
			n := ensure(pid)
			raw := strings.Trim(string(r.Value[1]), `"`)
			n.cpu, _ = strconv.ParseFloat(raw, 64)
			n.online = true
		}
	}
	valFor := func(res *vmResult, _ string) map[string]float64 {
		m := map[string]float64{}
		if res == nil {
			return m
		}
		for _, r := range res.Data.Result {
			pid := r.Metric["peer_id"]
			if len(r.Value) >= 2 {
				raw := strings.Trim(string(r.Value[1]), `"`)
				v, _ := strconv.ParseFloat(raw, 64)
				m[pid] += v
			}
		}
		return m
	}

	txMap := valFor(txR, "tx")
	rxMap := valFor(rxR, "rx")
	latMap := valFor(latR, "latency")

	for pid := range txMap {
		ensure(pid).txMbps = txMap[pid]
	}
	for pid := range rxMap {
		ensure(pid).rxMbps = rxMap[pid]
	}
	for pid := range latMap {
		ensure(pid).latency = latMap[pid]
	}

	result := make([]nodeInfo, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, *n)
	}
	return result
}

// ── checks ────────────────────────────────────────────────────────────────────

type check struct {
	name   string
	metric string
	promql func(ns string) string
}

var checks = []check{
	{
		name:   "节点心跳 (uptime)",
		metric: "wireflow_node_uptime_seconds",
		promql: func(ns string) string {
			return fmt.Sprintf(`count(last_over_time(wireflow_node_uptime_seconds{network_id="%s"}[5m]))`, ns)
		},
	},
	{
		name:   "CPU 使用率",
		metric: "wireflow_node_cpu_usage_percent",
		promql: func(ns string) string {
			return fmt.Sprintf(`count(last_over_time(wireflow_node_cpu_usage_percent{network_id="%s"}[5m]))`, ns)
		},
	},
	{
		name:   "内存用量",
		metric: "wireflow_node_memory_bytes",
		promql: func(ns string) string {
			return fmt.Sprintf(`count(last_over_time(wireflow_node_memory_bytes{network_id="%s"}[5m]))`, ns)
		},
	},
	{
		name:   "流量计数器 TX",
		metric: "wireflow_node_traffic_bytes_total{direction=tx}",
		promql: func(ns string) string {
			return fmt.Sprintf(`count(last_over_time(wireflow_node_traffic_bytes_total{network_id="%s",direction="tx"}[5m]))`, ns)
		},
	},
	{
		name:   "流量计数器 RX",
		metric: "wireflow_node_traffic_bytes_total{direction=rx}",
		promql: func(ns string) string {
			return fmt.Sprintf(`count(last_over_time(wireflow_node_traffic_bytes_total{network_id="%s",direction="rx"}[5m]))`, ns)
		},
	},
	{
		name:   "Peer 握手状态",
		metric: "wireflow_peer_status",
		promql: func(ns string) string {
			return fmt.Sprintf(`count(last_over_time(wireflow_peer_status{network_id="%s"}[5m]))`, ns)
		},
	},
	{
		name:   "ICMP 延迟",
		metric: "wireflow_peer_latency_ms",
		promql: func(ns string) string {
			return fmt.Sprintf(`count(last_over_time(wireflow_peer_latency_ms{network_id="%s"}[5m]))`, ns)
		},
	},
	{
		name:   "丢包率",
		metric: "wireflow_peer_packet_loss_percent",
		promql: func(ns string) string {
			return fmt.Sprintf(`count(last_over_time(wireflow_peer_packet_loss_percent{network_id="%s"}[5m]))`, ns)
		},
	},
}

// ── display ───────────────────────────────────────────────────────────────────

const (
	green  = "\033[32m"
	red    = "\033[31m"
	yellow = "\033[33m"
	reset  = "\033[0m"
	bold   = "\033[1m"
)

func colored(s, c string) string { return c + s + reset }

func run() bool {
	ns := *networkID
	fmt.Printf("\n%s=== Wireflow Metrics Verify  %s ===%s\n",
		bold, time.Now().Format("2006-01-02 15:04:05"), reset)
	fmt.Printf("VM:         %s\n", *vmURL)
	fmt.Printf("network_id: %s\n\n", ns)

	// ── metric existence checks ───────────────────────────────────────────
	fmt.Printf("%-36s  %s\n", "Metric", "Series")
	fmt.Println(strings.Repeat("─", 56))

	allOK := true
	for _, c := range checks {
		r, err := query(c.promql(ns))
		if err != nil {
			fmt.Printf("  %-34s  %s\n", c.name, colored("ERR: "+err.Error(), red))
			allOK = false
			continue
		}
		n := count(r)
		if n == 0 {
			fmt.Printf("  %-34s  %s\n", c.name, colored("NO DATA", red))
			allOK = false
		} else {
			fmt.Printf("  %-34s  %s\n", c.name,
				colored(fmt.Sprintf("OK  (%d series)", n), green))
		}
	}

	// ── aggregated rates ──────────────────────────────────────────────────
	txR, _ := query(fmt.Sprintf(
		`sum(irate(wireflow_node_traffic_bytes_total{network_id="%s",direction="tx"}[2m])) * 8 / 1e6`, ns))
	rxR, _ := query(fmt.Sprintf(
		`sum(irate(wireflow_node_traffic_bytes_total{network_id="%s",direction="rx"}[2m])) * 8 / 1e6`, ns))
	latR, _ := query(fmt.Sprintf(
		`avg(wireflow_peer_latency_ms{network_id="%s"})`, ns))
	lossR, _ := query(fmt.Sprintf(
		`avg(wireflow_peer_packet_loss_percent{network_id="%s"})`, ns))
	onlineR, _ := query(fmt.Sprintf(
		`count(last_over_time(wireflow_node_uptime_seconds{network_id="%s"}[5m]))`, ns))

	fmt.Println()
	fmt.Println(strings.Repeat("─", 56))
	fmt.Printf("  Online nodes : %s\n", bold+fmt.Sprintf("%.0f", scalar(onlineR))+reset)
	fmt.Printf("  TX rate      : %.3f Mbps\n", scalar(txR))
	fmt.Printf("  RX rate      : %.3f Mbps\n", scalar(rxR))
	fmt.Printf("  Avg latency  : %.1f ms\n", scalar(latR))
	fmt.Printf("  Avg loss     : %.2f %%\n", scalar(lossR))

	// ── per-node table ────────────────────────────────────────────────────
	nodes := fetchNodes(ns)
	if len(nodes) > 0 {
		fmt.Println()
		fmt.Printf("  %-24s  %6s  %8s  %8s  %7s\n",
			"Peer ID", "CPU%", "TX Mbps", "RX Mbps", "Lat ms")
		fmt.Println("  " + strings.Repeat("─", 60))
		for _, n := range nodes {
			status := colored("online", green)
			if !n.online {
				status = colored("offline", yellow)
			}
			fmt.Printf("  %-24s  %5.1f%%  %7.3f  %7.3f  %6.1f  %s\n",
				n.peerID, n.cpu, n.txMbps, n.rxMbps, n.latency, status)
		}
	}

	fmt.Println()
	if allOK {
		fmt.Println(colored("  All checks passed — dashboard should display data.", green))
	} else {
		fmt.Println(colored("  Some metrics missing. Check:", red))
		fmt.Println("    1. Nodes are running with --enable-metric --vm-endpoint <vm>/api/v1/write")
		fmt.Println("    2. This is a Pro build (community build has no telemetry)")
		fmt.Println("    3. --network-id matches t_workspace.namespace in the DB")
		fmt.Println("    4. VM is reachable from the nodes")
	}
	fmt.Println()

	return allOK
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	flag.Parse()
	if *networkID == "" {
		fmt.Fprintln(os.Stderr, "Error: --network-id is required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Find it with: SELECT namespace FROM t_workspace WHERE name='<ws>';")
		fmt.Fprintln(os.Stderr, "")
		flag.Usage()
		os.Exit(1)
	}

	if *watch {
		for {
			run()
			fmt.Printf("  (next check in 30s, Ctrl+C to quit)\n")
			time.Sleep(30 * time.Second)
		}
	} else {
		if !run() {
			os.Exit(1)
		}
	}
}
