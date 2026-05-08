package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/pkg/utils"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup"
)

type MonitorService interface {
	GetTopologySnapshot(ctx context.Context) ([]models.PeerSnapshot, error)
	// GetNodeSnapshot returns snapshots for all nodes in the given namespace (= network_id label).
	GetNodeSnapshot(ctx context.Context, namespace string) ([]models.NodeSnapshot, error)
	// GetWorkspaceAggregatedMonitor returns live stats for the given namespace.
	GetWorkspaceAggregatedMonitor(ctx context.Context, namespace string) (*models.AggregatedMonitorResponse, error)
	// GetWorkspaceDashboard returns a workspace-scoped dashboard response for the given namespace.
	GetWorkspaceDashboard(ctx context.Context, namespace string) (*models.WorkspaceDashboardResponse, error)
	GetGlobalDashboard(ctx context.Context) (*models.DashboardResponse, error)
}

type monitorService struct {
	api     v1.API
	log     *log.Logger
	timeout time.Duration
	cache   *cache.Cache
}

// ... existing code ...

type MonitorServiceOptions struct {
	// Address Prometheus / VictoriaMetrics PromQL API address
	// e.g. "http://localhost:8428"
	Address string

	// Timeout single query timeout; applies when ctx itself has no deadline set
	Timeout time.Duration

	// Logger optional: uses default logger if not provided
	Logger *log.Logger
}

func NewMonitorService(address string) (MonitorService, error) {
	// Backward-compatible signature: internally delegates to Options version
	return NewMonitorServiceWithOptions(MonitorServiceOptions{
		Address: address,
		Timeout: 5 * time.Second,
	})
}

func NewMonitorServiceWithOptions(opts MonitorServiceOptions) (MonitorService, error) {
	if opts.Address == "" {
		return nil, fmt.Errorf("monitor service: empty address")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = log.GetLogger("vm-service")
	}

	client, err := api.NewClient(api.Config{
		Address: opts.Address,
	})
	if err != nil {
		return nil, err
	}

	return &monitorService{
		api:     v1.NewAPI(client),
		log:     opts.Logger,
		timeout: opts.Timeout,
		cache:   cache.New(5*time.Minute, 10*time.Minute),
	}, nil
}

// ... existing code ...

//// ensureTimeout: if ctx has no deadline, inject the default timeout; otherwise return as-is
//func (v *monitorService) ensureTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
//	if _, ok := ctx.Deadline(); ok {
//		return ctx, func() {}
//	}
//	return context.WithTimeout(ctx, v.timeout)
//}
//
//// queryInstant executes a PromQL Instant Query and handles warnings uniformly
//func (v *monitorService) queryInstant(ctx context.Context, promql string, ts time.Time) (model.Value, error) {
//	ctx, cancel := v.ensureTimeout(ctx)
//	defer cancel()
//
//	val, warnings, err := v.api.Query(ctx, promql, ts)
//	if err != nil {
//		return nil, err
//	}
//	for _, w := range warnings {
//		// Avoid fmt.Printf, use logger uniformly
//		v.log.Warn("promql warning", "warning", w, "query", promql)
//	}
//	return val, nil
//}

// GetTopologySnapshot gets the topology status of all Peers
func (v *monitorService) GetTopologySnapshot(ctx context.Context) ([]models.PeerSnapshot, error) {
	// 1. Query all metrics starting with lattice_node_
	query := `last_over_time({__name__=~"lattice_node_.*"}[5m])`
	vector, err := v.QueryByTime(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	nodeMap := make(map[string]*models.PeerSnapshot)

	for _, s := range vector {
		nodeID := string(s.Metric["node_id"])
		metricName := string(s.Metric["__name__"])
		val := float64(s.Value)

		// Initialize node
		if _, ok := nodeMap[nodeID]; !ok {
			nodeMap[nodeID] = &models.PeerSnapshot{
				ID:          nodeID,
				Name:        string(s.Metric["node_id"]),
				InternalIP:  string(s.Metric["ip"]),
				Status:      "online",
				HealthLevel: "success",
				Metrics:     make(map[string]string),
			}
		}

		// 2. Auto-format and store in Map
		// We strip the "lattice_node_" prefix so keys exposed to the frontend are cleaner
		shortName := strings.TrimPrefix(metricName, "lattice_node_")
		nodeMap[nodeID].Metrics[shortName] = utils.AutoFormat(metricName, val)

		// 3. Special logic: determine health level based on CPU
		if shortName == "cpu_usage_percent" {
			if val > 80 {
				nodeMap[nodeID].HealthLevel = "warning"
			}
			if val > 95 {
				nodeMap[nodeID].HealthLevel = "error"
			}
		}
	}

	// Convert to slice
	var result []models.PeerSnapshot
	for _, node := range nodeMap {
		result = append(result, *node)
	}
	return result, nil
}

// QueryByTime executes an Instant Query
// query: PromQL statement, e.g. `last_over_time(peer_status[5m])`
// t: target time. Pass time.Now() for current, or a past timestamp for historical data.
func (v *monitorService) QueryByTime(ctx context.Context, query string, t time.Time) (model.Vector, error) {
	// 1. Call the underlying v1.API. Note: the Query interface returns a “snapshot” at time t
	result, warnings, err := v.api.Query(ctx, query, t)
	if err != nil {
		return nil, fmt.Errorf("promql query error: %v", err)
	}

	// 2. Print potential warnings returned by VM (e.g. query timeout, partial data)
	for _, w := range warnings {
		fmt.Printf("VM Warning: %v\n", w)
	}

	// 3. Type assertion. Instant Query results are typically a Vector
	// If you query a non-existent metric, an empty Vector is returned rather than an error
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T, expected model.Vector", result)
	}

	return vector, nil
}

// GetNodeSnapshot gets the node snapshot for a specific namespace
func (s *monitorService) GetNodeSnapshot(ctx context.Context, namespace string) ([]models.NodeSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	query := fmt.Sprintf(`last_over_time({network_id="%s"}[5m])`, namespace)

	// Execute Instant Query
	val, _, err := s.api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	vector, ok := val.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected prometheus return type")
	}

	// 2. Transform data structure
	// Use Map to aggregate different metrics for the same node
	nodeMap := make(map[string]*models.NodeSnapshot)

	for _, sample := range vector {
		nodeID := string(sample.Metric["node_id"])
		if _, exists := nodeMap[nodeID]; !exists {
			nodeMap[nodeID] = &models.NodeSnapshot{
				ID:         nodeID,
				Name:       string(sample.Metric["node_name"]),
				IP:         string(sample.Metric["node_ip"]),
				Metrics:    make(map[string]string),
				RawMetrics: make(map[string]float64),
				Status:     "online",
			}
		}

		metricName := string(sample.Metric["__name__"])
		value := float64(sample.Value)

		// 3. Flexibly fill in metrics
		s.fillMetrics(nodeMap[nodeID], metricName, value)
	}

	// Convert to slice and return
	result := make([]models.NodeSnapshot, 0, len(nodeMap))
	for _, v := range nodeMap {
		result = append(result, *v)
	}
	return result, nil
}

// fillMetrics maps raw monitoring items to business fields
func (s *monitorService) fillMetrics(node *models.NodeSnapshot, name string, val float64) {
	switch name {
	case models.WIREWFLOW_NODE_CPU_USEAGE:
		node.RawMetrics["cpu"] = val
		node.Metrics["cpu"] = fmt.Sprintf("%.1f%%", val)
		// Dynamic logic: mark as error when CPU exceeds 90%
		if val > 90 {
			node.HealthLevel = "error"
		}
	case models.LATTICE_PEER_STATUS:
		if val == 1 {
			node.Status = "online"
			if node.HealthLevel == "" {
				node.HealthLevel = "success"
			}
		} else {
			node.Status = "offline"
			node.HealthLevel = "error"
		}
	// You can add unlimited new monitoring items here, such as gpu_temp, mem_usage, etc.
	default:
		node.RawMetrics[name] = val
		node.Metrics[name] = fmt.Sprintf("%.2f", val)
	}
}

// GetGlobalStats gets globally aggregated metrics
func (s *monitorService) GetGlobalStats(ctx context.Context, metricName string) (map[string]float64, error) {
	// Use sum(...) by (workspace_id) for server-side aggregation
	query := fmt.Sprintf(`sum(%s) by (workspace_id)`, metricName)

	val, _, err := s.api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	vector, ok := val.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected vector type")
	}

	stats := make(map[string]float64)
	for _, sample := range vector {
		wsID := string(sample.Metric["workspace_id"])
		stats[wsID] = float64(sample.Value)
	}

	return stats, nil
}

func (s *monitorService) GetWorkspaceAggregatedMonitor(ctx context.Context, namespace string) (*models.AggregatedMonitorResponse, error) {
	var eg errgroup.Group
	resp := &models.AggregatedMonitorResponse{
		WorkspaceID: namespace,
		LiveStats:   make([]models.StatCard, 4),
	}

	// 1. Get real-time throughput (TX)
	eg.Go(func() error {
		resp.LiveStats[0] = s.fetchThroughput(ctx, namespace)
		return nil
	})

	// 2. Get average latency
	eg.Go(func() error {
		query := fmt.Sprintf(`avg(lattice_peer_latency_ms{network_id="%s"})`, namespace)
		val, _, err := s.api.Query(ctx, query, time.Now())
		if err == nil {
			resp.LiveStats[1] = models.StatCard{
				Label: "Average Latency",
				Value: s.formatVectorValue(val),
				Unit:  "ms",
				Color: "text-emerald-500",
			}
		}
		return err
	})

	// 3. Get packet loss rate
	eg.Go(func() error {
		query := fmt.Sprintf(`avg(lattice_peer_packet_loss_percent{network_id="%s"})`, namespace)
		val, _, err := s.api.Query(ctx, query, time.Now())
		if err == nil {
			resp.LiveStats[2] = models.StatCard{
				Label: "Packet Loss",
				Value: s.formatVectorValue(val),
				Unit:  "%",
				Color: "text-emerald-500",
			}
		}
		return err
	})

	// 4. Active tunnels: peer_status==1 connections / 2 (each tunnel reports from both ends)
	eg.Go(func() error {
		query := fmt.Sprintf(`ceil(sum(lattice_peer_status{network_id="%s"} == 1) / 2)`, namespace)
		val, _, err := s.api.Query(ctx, query, time.Now())
		if err == nil {
			resp.LiveStats[3] = models.StatCard{
				Label: "Active Tunnels",
				Value: s.formatVectorValue(val),
				Unit:  "TUNNELS",
				Color: "text-emerald-500",
			}
		}
		return err
	})

	// 5. Throughput trend (past 1h, 2m granularity, TX + RX)
	eg.Go(func() error {
		r := v1.Range{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now(),
			Step:  time.Minute * 2,
		}
		txQuery := fmt.Sprintf(`sum(irate(lattice_node_traffic_bytes_total{network_id="%s",direction="tx"}[5m])) * 8 / 1e6`, namespace)
		rxQuery := fmt.Sprintf(`sum(irate(lattice_node_traffic_bytes_total{network_id="%s",direction="rx"}[5m])) * 8 / 1e6`, namespace)
		txResult, _, err := s.api.QueryRange(ctx, txQuery, r)
		if err == nil {
			rxResult, _, _ := s.api.QueryRange(ctx, rxQuery, r)
			resp.Trend = s.processMatrixToTrendWithRX(txResult, rxResult)
		}
		return err
	})

	// 6. Node list details
	eg.Go(func() error {
		query := fmt.Sprintf(`last_over_time(lattice_peer_status{network_id="%s"}[5m])`, namespace)
		val, _, err := s.api.Query(ctx, query, time.Now())
		if err == nil {
			resp.Nodes = s.convertVectorToNodes(val)
		}
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetWorkspaceDashboard workspace-scoped Dashboard: concurrently queries VM, returns 4 stat cards + throughput trend + node CPU + Top nodes.
// namespace corresponds to the network_id label value in VictoriaMetrics (i.e. workspace.Namespace).
func (s *monitorService) GetWorkspaceDashboard(ctx context.Context, namespace string) (*models.WorkspaceDashboardResponse, error) {
	var (
		eg    errgroup.Group
		mu    sync.Mutex
		cards = make([]models.WorkspaceStatCard, 4) // 0:nodes 1:throughput 2:latency 3:packet loss
		resp  = &models.WorkspaceDashboardResponse{}
	)

	// 0. Online node count
	eg.Go(func() error {
		q := fmt.Sprintf(`count(last_over_time(lattice_node_uptime_seconds{network_id="%s"}[5m]))`, namespace)
		vec, _ := s.QueryByTime(ctx, q, time.Now())
		val := 0
		if len(vec) > 0 {
			val = int(vec[0].Value)
		}
		cards[0] = models.WorkspaceStatCard{
			Label: "Online Nodes", Value: strconv.Itoa(val), Unit: "NODES",
			Trend: "stable", Color: "text-emerald-500",
		}
		return nil
	})

	// 1. Real-time throughput TX (Mbps)
	eg.Go(func() error {
		q := fmt.Sprintf(`sum(irate(lattice_node_traffic_bytes_total{network_id="%s",direction="tx"}[2m])) * 8 / 1e6`, namespace)
		vec, _ := s.QueryByTime(ctx, q, time.Now())
		val := 0.0
		if len(vec) > 0 {
			val = float64(vec[0].Value)
		}
		cards[1] = models.WorkspaceStatCard{
			Label: "Real-time Throughput", Value: fmt.Sprintf("%.1f", val), Unit: "Mbps",
			Trend: s.getTrend(namespace+"_tx", val), Color: "text-blue-500",
		}
		return nil
	})

	// 2. Average latency (ms)
	eg.Go(func() error {
		q := fmt.Sprintf(`avg(lattice_peer_latency_ms{network_id="%s"})`, namespace)
		vec, _ := s.QueryByTime(ctx, q, time.Now())
		val := 0.0
		if len(vec) > 0 {
			val = float64(vec[0].Value)
		}
		trend := "stable"
		if val > 100 {
			trend = "up"
		}
		cards[2] = models.WorkspaceStatCard{
			Label: "Average Latency", Value: fmt.Sprintf("%.1f", val), Unit: "ms",
			Trend: trend, Color: "text-amber-500",
		}
		return nil
	})

	// 3. Average packet loss (%)
	eg.Go(func() error {
		q := fmt.Sprintf(`avg(lattice_peer_packet_loss_percent{network_id="%s"})`, namespace)
		vec, _ := s.QueryByTime(ctx, q, time.Now())
		val := 0.0
		if len(vec) > 0 {
			val = float64(vec[0].Value)
		}
		trend := "stable"
		if val > 1 {
			trend = "up"
		}
		cards[3] = models.WorkspaceStatCard{
			Label: "Packet Loss", Value: fmt.Sprintf("%.2f", val), Unit: "%",
			Trend: trend, Color: "text-emerald-500",
		}
		return nil
	})

	// 4. Throughput trend (last 1h, 2m granularity, TX + RX, unit Mbps)
	eg.Go(func() error {
		r := v1.Range{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now(),
			Step:  2 * time.Minute,
		}
		txQ := fmt.Sprintf(`sum(irate(lattice_node_traffic_bytes_total{network_id="%s",direction="tx"}[5m])) * 8 / 1e6`, namespace)
		rxQ := fmt.Sprintf(`sum(irate(lattice_node_traffic_bytes_total{network_id="%s",direction="rx"}[5m])) * 8 / 1e6`, namespace)
		txResult, _, _ := s.api.QueryRange(ctx, txQ, r)
		rxResult, _, _ := s.api.QueryRange(ctx, rxQ, r)
		trend := s.processMatrixToTrendWithRX(txResult, rxResult)
		mu.Lock()
		resp.ThroughputTrend = trend
		mu.Unlock()
		return nil
	})

	// 5. Node CPU + Memory
	eg.Go(func() error {
		cpuQ := fmt.Sprintf(`last_over_time(lattice_node_cpu_usage_percent{network_id="%s"}[5m])`, namespace)
		memQ := fmt.Sprintf(`last_over_time(lattice_node_memory_bytes{network_id="%s"}[5m])`, namespace)
		cpuVec, _ := s.QueryByTime(ctx, cpuQ, time.Now())
		memVec, _ := s.QueryByTime(ctx, memQ, time.Now())

		memMap := make(map[string]float64, len(memVec))
		for _, samp := range memVec {
			memMap[string(samp.Metric["peer_id"])] = float64(samp.Value) / 1e6
		}

		items := make([]models.NodeCPUItem, 0, len(cpuVec))
		for _, samp := range cpuVec {
			pid := string(samp.Metric["peer_id"])
			items = append(items, models.NodeCPUItem{
				PeerID:   pid,
				Name:     pid,
				CPU:      float64(samp.Value),
				MemoryMB: memMap[pid],
			})
		}
		mu.Lock()
		resp.NodeCPU = items
		mu.Unlock()
		return nil
	})

	// 6. Top 10 nodes (24h traffic)
	eg.Go(func() error {
		trafficQ := fmt.Sprintf(
			`topk(10, sum by (peer_id)(increase(lattice_node_traffic_bytes_total{network_id="%s"}[24h])))`,
			namespace)
		trafficVec, _ := s.QueryByTime(ctx, trafficQ, time.Now())

		statusQ := fmt.Sprintf(`last_over_time(lattice_peer_status{network_id="%s"}[5m])`, namespace)
		statusVec, _ := s.QueryByTime(ctx, statusQ, time.Now())

		onlineMap := make(map[string]bool)
		endpointMap := make(map[string]string)
		for _, samp := range statusVec {
			pid := string(samp.Metric["peer_id"])
			if float64(samp.Value) == 1 {
				onlineMap[pid] = true
			}
			if ep := string(samp.Metric["endpoint"]); ep != "" && endpointMap[pid] == "" {
				endpointMap[pid] = ep
			}
		}

		nodes := make([]models.NodeMonitorDetail, 0, len(trafficVec))
		for _, samp := range trafficVec {
			pid := string(samp.Metric["peer_id"])
			nodes = append(nodes, models.NodeMonitorDetail{
				ID:       pid,
				Name:     pid,
				Endpoint: endpointMap[pid],
				Online:   onlineMap[pid],
				TotalTx:  int64(float64(samp.Value)),
			})
		}
		mu.Lock()
		resp.TopNodes = nodes
		mu.Unlock()
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	resp.StatCards = cards
	return resp, nil
}

// formatVectorValue formats a scalar value
func (s *monitorService) formatVectorValue(val model.Value) string {
	vector, ok := val.(model.Vector)
	if !ok || len(vector) == 0 {
		return "0.0"
	}
	return fmt.Sprintf("%.1f", float64(vector[0].Value))
}

// processMatrixToTrend converts a Range Query Matrix to frontend waveform format
// nolint:unused
func (s *monitorService) processMatrixToTrend(val model.Value) models.TrendData {
	return s.processMatrixToTrendWithRX(val, nil)
}

// processMatrixToTrendWithRX fills both TX and RX trend data
func (s *monitorService) processMatrixToTrendWithRX(txVal model.Value, rxVal model.Value) models.TrendData {
	trend := models.TrendData{
		Timestamps: []string{},
		TXData:     []float64{},
		RXData:     []float64{},
	}

	txMatrix, ok := txVal.(model.Matrix)
	if ok && len(txMatrix) > 0 {
		for _, sample := range txMatrix[0].Values {
			trend.Timestamps = append(trend.Timestamps, sample.Timestamp.Time().Format("15:04"))
			trend.TXData = append(trend.TXData, float64(sample.Value))
		}
	}

	rxMatrix, ok := rxVal.(model.Matrix)
	if ok && len(rxMatrix) > 0 {
		for _, sample := range rxMatrix[0].Values {
			trend.RXData = append(trend.RXData, float64(sample.Value))
		}
	}

	return trend
}

// convertVectorToNodes converts node label information to a detail list
// Data source: lattice_peer_status, labels: workspace_id, node_id, peer_id, endpoint, alias
func (s *monitorService) convertVectorToNodes(val model.Value) []models.NodeMonitorDetail {
	vector, _ := val.(model.Vector)
	nodes := make([]models.NodeMonitorDetail, 0)
	for _, sample := range vector {
		online := float64(sample.Value) == 1
		nodes = append(nodes, models.NodeMonitorDetail{
			ID:       string(sample.Metric["peer_id"]),
			Name:     string(sample.Metric["node_id"]),
			Endpoint: string(sample.Metric["endpoint"]),
			Online:   online,
		})
	}
	return nodes
}

func (s *monitorService) fetchThroughput(ctx context.Context, namespace string) models.StatCard {
	query := fmt.Sprintf(`sum(irate(lattice_node_traffic_bytes_total{network_id="%s",direction="tx"}[2m])) * 8 / 1e6`, namespace)

	// 2. Execute query
	val, _, err := s.api.Query(ctx, query, time.Now())

	if err != nil {
		return models.StatCard{Label: "Real-time Throughput", Value: "0.0", Unit: "Mbps", Trend: "stable", Color: "text-blue-500"}
	}

	vec, _ := val.(model.Vector)
	if len(vec) == 0 {
		return models.StatCard{Label: "Real-time Throughput", Value: "0.0", Unit: "Mbps", Trend: "stable", Color: "text-blue-500"}
	}

	currentValue := float64(vec[0].Value)
	percent := int((currentValue / 1000.0) * 100)
	if percent > 100 {
		percent = 100
	}

	return models.StatCard{
		Label:   "Real-time Throughput",
		Value:   fmt.Sprintf("%.1f", currentValue),
		Unit:    "Mbps",
		Trend:   s.getTrend(namespace, currentValue),
		Color:   "text-blue-500",
		Percent: percent,
	}
}

func (s *monitorService) getTrend(wsID string, current float64) string {
	lastVal, exists := s.cache.Get("last_tp_" + wsID)
	s.cache.Set("last_tp_"+wsID, current, 1*time.Minute)

	if !exists {
		return "stable"
	}
	if current > lastVal.(float64)*1.05 {
		return "up"
	} // Growth exceeding 5% is considered an increase
	if current < lastVal.(float64)*0.95 {
		return "down"
	} // Drop exceeding 5% is considered a decrease
	return "stable"
}

// GetGlobalDashboard concurrently queries VM, aggregates global Dashboard data
func (s *monitorService) GetGlobalDashboard(ctx context.Context) (*models.DashboardResponse, error) {
	var (
		eg             errgroup.Group
		mu             sync.Mutex
		activeWs       int
		onlineNodes    int
		throughputGbps float64
		wsNodes        = make(map[string]int)
		wsTraffic      = make(map[string]float64)
		wsHealth       = make(map[string]float64)
		globalTrend    models.TrendData
		topNodes       []models.NodeMonitorDetail
	)

	// 1. Active workspace count (workspaces with data reported)
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `count(count by (workspace_id) (lattice_peer_status{workspace_id!=""}))`, time.Now())
		if err == nil && len(vec) > 0 {
			activeWs = int(vec[0].Value)
		}
		return nil
	})

	// 2. Total online nodes across all networks (deduplicated by node_id)
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `count(count by (peer_id) (lattice_peer_status == 1))`, time.Now())
		if err == nil && len(vec) > 0 {
			onlineNodes = int(vec[0].Value)
		}
		return nil
	})

	// 3. Global total throughput (Gbps)
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `sum(irate(lattice_peer_traffic_bytes_total{direction="tx"}[2m])) * 8 / 1e9`, time.Now())
		if err == nil && len(vec) > 0 {
			throughputGbps = float64(vec[0].Value)
		}
		return nil
	})

	// 4. Online node count per workspace (grouped by network_id)
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `count by (network_id) (last_over_time(lattice_node_uptime_seconds[5m]))`, time.Now())
		if err != nil {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		for _, sample := range vec {
			nsID := string(sample.Metric["network_id"])
			wsNodes[nsID] = int(sample.Value)
		}
		return nil
	})

	// 5. 24h sent traffic per workspace (aggregated at node level to avoid double counting)
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `sum by (network_id) (increase(lattice_node_traffic_bytes_total{direction="tx"}[24h]))`, time.Now())
		if err != nil {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		for _, sample := range vec {
			nsID := string(sample.Metric["network_id"])
			wsTraffic[nsID] = float64(sample.Value)
		}
		return nil
	})

	// 6. Health score per workspace (percentage of online peers)
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `avg by (network_id) (lattice_peer_status) * 100`, time.Now())
		if err != nil {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		for _, sample := range vec {
			nsID := string(sample.Metric["network_id"])
			wsHealth[nsID] = float64(sample.Value)
		}
		return nil
	})

	// 7. Global throughput trend (from midnight today to now, 4h granularity, up to 6 points)
	eg.Go(func() error {
		midnight := time.Now().Truncate(24 * time.Hour)
		r := v1.Range{
			Start: midnight,
			End:   time.Now(),
			Step:  4 * time.Hour,
		}
		txQ := `sum(irate(lattice_node_traffic_bytes_total{direction="tx"}[20m])) * 8 / 1e9`
		rxQ := `sum(irate(lattice_node_traffic_bytes_total{direction="rx"}[20m])) * 8 / 1e9`
		txResult, _, _ := s.api.QueryRange(ctx, txQ, r)
		rxResult, _, _ := s.api.QueryRange(ctx, rxQ, r)
		trend := s.processMatrixToTrendWithRX(txResult, rxResult)
		mu.Lock()
		globalTrend = trend
		mu.Unlock()
		return nil
	})

	// 8. Top 10 nodes (24h traffic) + CPU + online status
	eg.Go(func() error {
		trafficVec, err := s.QueryByTime(ctx,
			`topk(10, sum by (peer_id, network_id) (increase(lattice_node_traffic_bytes_total[24h])))`,
			time.Now())
		if err != nil {
			return nil
		}

		cpuVec, _ := s.QueryByTime(ctx, `last_over_time(lattice_node_cpu_usage_percent[5m])`, time.Now())
		cpuMap := make(map[string]float64)
		for _, samp := range cpuVec {
			cpuMap[string(samp.Metric["peer_id"])] = float64(samp.Value)
		}

		statusVec, _ := s.QueryByTime(ctx, `last_over_time(lattice_peer_status[5m])`, time.Now())
		onlineMap := make(map[string]bool)
		endpointMap := make(map[string]string)
		for _, samp := range statusVec {
			pid := string(samp.Metric["peer_id"])
			if float64(samp.Value) == 1 {
				onlineMap[pid] = true
			}
			if ep := string(samp.Metric["endpoint"]); ep != "" && endpointMap[pid] == "" {
				endpointMap[pid] = ep
			}
		}

		seen := make(map[string]bool)
		nodes := make([]models.NodeMonitorDetail, 0, len(trafficVec))
		for _, samp := range trafficVec {
			pid := string(samp.Metric["peer_id"])
			if seen[pid] {
				continue
			}
			seen[pid] = true
			nodes = append(nodes, models.NodeMonitorDetail{
				ID:       pid,
				Name:     pid,
				Endpoint: endpointMap[pid],
				Online:   onlineMap[pid],
				CPU:      cpuMap[pid],
				TotalTx:  int64(float64(samp.Value)),
			})
		}
		mu.Lock()
		topNodes = nodes
		mu.Unlock()
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	resp := &models.DashboardResponse{
		GlobalStats: []models.GlobalStatItem{
			{
				Label: "Active Workspaces", Value: strconv.Itoa(activeWs), Unit: "SETS",
				Trend: "+0", Color: "text-blue-500", BarWidth: calcProgress(activeWs, 20), TrendUp: true,
			},
			{
				Label: "Global Online Nodes", Value: strconv.Itoa(onlineNodes), Unit: "NODE",
				Trend: "Live", Color: "text-emerald-500", BarWidth: calcProgress(onlineNodes, 2000), TrendUp: true,
			},
			{
				Label: "Global Total Throughput", Value: fmt.Sprintf("%.1f", throughputGbps), Unit: "Gbps",
				Trend: "Gbps", Color: "text-primary", BarWidth: calcProgress(int(throughputGbps*10), 100), TrendUp: true,
			},
			{
				Label: "Unhandled Alerts", Value: "00", Unit: "WARN",
				Trend: "Healthy", Color: "text-error", BarWidth: "0%", TrendUp: false,
			},
		},
		GlobalEvents: []models.GlobalEventItem{},
	}

	resp.GlobalTrend = globalTrend
	resp.TopNodes = topNodes

	// Merge workspace data (based on workspaces with traffic or nodes)
	wsIDs := make(map[string]struct{})
	mu.Lock()
	for id := range wsNodes {
		wsIDs[id] = struct{}{}
	}
	for id := range wsTraffic {
		wsIDs[id] = struct{}{}
	}
	mu.Unlock()

	for wsID := range wsIDs {
		health := int(wsHealth[wsID])
		if health == 0 {
			health = 100
		}
		status := "Running"
		if health < 90 {
			status = "Warning"
		}
		resp.WorkspaceUsage = append(resp.WorkspaceUsage, models.WorkspaceUsageRow{
			Name:    wsID, // TODO: later join DB to replace with displayName
			Type:    "Production",
			Nodes:   wsNodes[wsID],
			Traffic: formatTrafficBytes(wsTraffic[wsID]),
			Health:  health,
			Status:  status,
		})
	}

	return resp, nil
}

// calcProgress maps value/max to a percentage string, e.g. "65%"
func calcProgress(value, max int) string {
	if max <= 0 || value <= 0 {
		return "0%"
	}
	pct := value * 100 / max
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf("%d%%", pct)
}

// formatTrafficBytes formats byte counts into a human-readable string
func formatTrafficBytes(b float64) string {
	switch {
	case b >= 1e12:
		return fmt.Sprintf("%.1f TB", b/1e12)
	case b >= 1e9:
		return fmt.Sprintf("%.1f GB", b/1e9)
	case b >= 1e6:
		return fmt.Sprintf("%.1f MB", b/1e6)
	default:
		return fmt.Sprintf("%.0f B", b)
	}
}
