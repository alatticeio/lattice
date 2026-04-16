package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"wireflow/internal/log"
	"wireflow/management/models"
	"wireflow/pkg/utils"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup"
)

type MonitorService interface {
	GetTopologySnapshot(ctx context.Context) ([]models.PeerSnapshot, error)
	GetNodeSnapshot(ctx context.Context, wsID string) ([]models.NodeSnapshot, error)
	GetWorkspaceAggregatedMonitor(ctx context.Context, wsID string) (*models.AggregatedMonitorResponse, error)
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
	// Address Prometheus / VictoriaMetrics PromQL API 地址
	// 例："http://localhost:8428"
	Address string

	// Timeout 单次查询超时；当 ctx 本身未设置 deadline 时生效
	Timeout time.Duration

	// Logger 可选：不传则使用默认 logger
	Logger *log.Logger
}

func NewMonitorService(address string) (MonitorService, error) {
	// 兼容旧签名：内部转到 Options 版本
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
	}, nil
}

// ... existing code ...

//// ensureTimeout：如果 ctx 没有 deadline，则注入默认超时；否则原样返回
//func (v *monitorService) ensureTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
//	if _, ok := ctx.Deadline(); ok {
//		return ctx, func() {}
//	}
//	return context.WithTimeout(ctx, v.timeout)
//}
//
//// queryInstant 执行一次 PromQL Instant Query，并统一处理 warnings
//func (v *monitorService) queryInstant(ctx context.Context, promql string, ts time.Time) (model.Value, error) {
//	ctx, cancel := v.ensureTimeout(ctx)
//	defer cancel()
//
//	val, warnings, err := v.api.Query(ctx, promql, ts)
//	if err != nil {
//		return nil, err
//	}
//	for _, w := range warnings {
//		// 避免 fmt.Printf，统一走 logger
//		v.log.Warn("promql warning", "warning", w, "query", promql)
//	}
//	return val, nil
//}

// GetPeerStatus 获取所有 Peer 的拓扑状态
func (v *monitorService) GetTopologySnapshot(ctx context.Context) ([]models.PeerSnapshot, error) {
	// 1. 查询所有以 wireflow_node_ 开头的指标
	query := `last_over_time({__name__=~"wireflow_node_.*"}[5m])`
	vector, err := v.QueryByTime(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	nodeMap := make(map[string]*models.PeerSnapshot)

	for _, s := range vector {
		nodeID := string(s.Metric["node_id"])
		metricName := string(s.Metric["__name__"])
		val := float64(s.Value)

		// 初始化节点
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

		// 2. 自动格式化并存入 Map
		// 我们去掉前缀 "wireflow_node_" 让前端拿到的 Key 更简洁
		shortName := strings.TrimPrefix(metricName, "wireflow_node_")
		nodeMap[nodeID].Metrics[shortName] = utils.AutoFormat(metricName, val)

		// 3. 特殊逻辑：根据 CPU 自动判定健康度
		if shortName == "cpu_usage_percent" {
			if val > 80 {
				nodeMap[nodeID].HealthLevel = "warning"
			}
			if val > 95 {
				nodeMap[nodeID].HealthLevel = "error"
			}
		}
	}

	// 转为切片
	var result []models.PeerSnapshot
	for _, node := range nodeMap {
		result = append(result, *node)
	}
	return result, nil
}

// QueryByTime 执行瞬时查询 (Instant Query)
// query: PromQL 语句，例如 `last_over_time(peer_status[5m])`
// t: 目标时间点。传入 time.Now() 查当前，传入过去的时间戳则查历史。
func (v *monitorService) QueryByTime(ctx context.Context, query string, t time.Time) (model.Vector, error) {
	// 1. 调用底层的 v1.API。注意：Query 接口返回的是指定时间点 t 的“快照”
	result, warnings, err := v.api.Query(ctx, query, t)
	if err != nil {
		return nil, fmt.Errorf("promql query error: %v", err)
	}

	// 2. 打印 VM 返回的潜在警告（如查询超时、数据部分缺失）
	for _, w := range warnings {
		fmt.Printf("VM Warning: %v\n", w)
	}

	// 3. 类型断言。Instant Query 的结果通常是 Vector (瞬时向量)
	// 如果你查的是一个不存在的指标，这里会返回一个空的 Vector 而不是 error
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T, expected model.Vector", result)
	}

	return vector, nil
}

// GetNodeSnapshots 获取特定空间的节点快照
func (s *monitorService) GetNodeSnapshot(ctx context.Context, wsID string) ([]models.NodeSnapshot, error) {
	// GetSnapshotsByPrometheus 从 VM 获取实时快照
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// 1. 定义 PromQL：查询该空间下所有节点的最新 CPU、内存和在线状态
	// 假设我们在 vmagent 上传时打上了 workspace_id 标签
	query := fmt.Sprintf(`last_over_time({node_id="%s"}[5m])`, "macbook-pro.local")

	// 执行即时查询 (Instant Query)
	val, _, err := s.api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	vector, ok := val.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected prometheus return type")
	}

	// 2. 转换数据结构
	// 使用 Map 聚合同一节点的不同指标
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

		// 3. 灵活填充指标
		s.fillMetrics(nodeMap[nodeID], metricName, value)
	}

	// 转为 Slice 返回
	result := make([]models.NodeSnapshot, 0, len(nodeMap))
	for _, v := range nodeMap {
		result = append(result, *v)
	}
	return result, nil
}

// fillMetrics 负责将原始监控项映射到业务字段
func (s *monitorService) fillMetrics(node *models.NodeSnapshot, name string, val float64) {
	switch name {
	case models.WIREWFLOW_NODE_CPU_USEAGE:
		node.RawMetrics["cpu"] = val
		node.Metrics["cpu"] = fmt.Sprintf("%.1f%%", val)
		// 动态逻辑：CPU 超过 90% 标记为 error
		if val > 90 {
			node.HealthLevel = "error"
		}
	case models.WIREFLOW_PEER_STATUS:
		if val == 1 {
			node.Status = "online"
			if node.HealthLevel == "" {
				node.HealthLevel = "success"
			}
		} else {
			node.Status = "offline"
			node.HealthLevel = "error"
		}
	// 你可以在这里无限增加新的监控项，如 gpu_temp, mem_usage 等
	default:
		node.RawMetrics[name] = val
		node.Metrics[name] = fmt.Sprintf("%.2f", val)
	}
}

// GetGlobalStats 获取全域聚合指标
func (s *monitorService) GetGlobalStats(ctx context.Context, metricName string) (map[string]float64, error) {
	// 使用 sum(...) by (workspace_id) 进行服务端聚合
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

func (s *monitorService) GetWorkspaceAggregatedMonitor(ctx context.Context, wsID string) (*models.AggregatedMonitorResponse, error) {
	var eg errgroup.Group
	resp := &models.AggregatedMonitorResponse{
		WorkspaceID: wsID,
		LiveStats:   make([]models.StatCard, 4), // 预分配固定长度
	}

	// 1. 获取实时吞吐量 (TX/RX)
	eg.Go(func() error {
		resp.LiveStats[0] = s.fetchThroughput(ctx, wsID)
		return nil
	})

	// 2. 获取平均延迟
	eg.Go(func() error {
		query := fmt.Sprintf(`avg(wireflow_peer_latency_ms{workspace_id="%s"})`, wsID)
		val, _, err := s.api.Query(ctx, query, time.Now())
		if err == nil {
			resp.LiveStats[1] = models.StatCard{
				Label: "平均延迟",
				Value: s.formatVectorValue(val),
				Unit:  "ms",
				Color: "text-emerald-500",
			}
		}
		return err
	})

	// 3. 获取丢包率
	eg.Go(func() error {
		query := fmt.Sprintf(`avg(wireflow_peer_packet_loss_percent{workspace_id="%s"})`, wsID)
		val, _, err := s.api.Query(ctx, query, time.Now())
		if err == nil {
			resp.LiveStats[2] = models.StatCard{
				Label: "丢包率",
				Value: s.formatVectorValue(val),
				Unit:  "%",
				Color: "text-emerald-500",
			}
		}
		return err
	})

	// 4. 获取活动隧道数
	eg.Go(func() error {
		query := fmt.Sprintf(`sum(wireflow_workspace_tunnels{workspace_id="%s",status="established"})`, wsID)
		val, _, err := s.api.Query(ctx, query, time.Now())
		if err == nil {
			resp.LiveStats[3] = models.StatCard{
				Label: "活动隧道",
				Value: s.formatVectorValue(val),
				Unit:  "条",
				Color: "text-emerald-500",
			}
		}
		return err
	})

	// 5. 获取趋势图数据 (过去 1 小时，TX + RX)
	eg.Go(func() error {
		r := v1.Range{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now(),
			Step:  time.Minute * 2,
		}
		txQuery := fmt.Sprintf(`sum(irate(wireflow_peer_traffic_bytes_total{workspace_id="%s",direction="tx"}[5m]))`, wsID)
		rxQuery := fmt.Sprintf(`sum(irate(wireflow_peer_traffic_bytes_total{workspace_id="%s",direction="rx"}[5m]))`, wsID)
		txResult, _, err := s.api.QueryRange(ctx, txQuery, r)
		if err == nil {
			rxResult, _, _ := s.api.QueryRange(ctx, rxQuery, r)
			resp.Trend = s.processMatrixToTrendWithRX(txResult, rxResult)
		}
		return err
	})

	// 6. 获取节点列表明细
	eg.Go(func() error {
		query := fmt.Sprintf(`last_over_time(wireflow_peer_status{workspace_id="%s"}[5m])`, wsID)
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

// 格式化标量值
func (s *monitorService) formatVectorValue(val model.Value) string {
	vector, ok := val.(model.Vector)
	if !ok || len(vector) == 0 {
		return "0.0"
	}
	return fmt.Sprintf("%.1f", float64(vector[0].Value))
}

// 将 Range Query 的 Matrix 转换为前端波形图格式
// nolint:unused
func (s *monitorService) processMatrixToTrend(val model.Value) models.TrendData {
	return s.processMatrixToTrendWithRX(val, nil)
}

// processMatrixToTrendWithRX 同时填充 TX 和 RX 趋势数据
func (s *monitorService) processMatrixToTrendWithRX(txVal model.Value, rxVal model.Value) models.TrendData {
	trend := models.TrendData{}

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

// 将节点标签信息转换为明细列表
// 数据来源：wireflow_peer_status，labels: workspace_id, node_id, peer_id, endpoint, alias
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

func (s *monitorService) fetchThroughput(ctx context.Context, wsID string) models.StatCard {
	// 1. 定义查询语句
	// 使用 irate 获取瞬时速率
	query := fmt.Sprintf(`sum(irate(wireflow_peer_traffic_bytes_total{workspace_id="%s",direction="tx"}[1m])) * 8 / 1000000`, wsID)

	// 2. 执行查询
	val, _, err := s.api.Query(ctx, query, time.Now())

	// 3. 默认值处理
	if err != nil || len(val.(model.Vector)) == 0 {
		return models.StatCard{
			Label:   "实时吞吐",
			Value:   "0.0",
			Unit:    "Mbps",
			Trend:   "stable",
			Color:   "text-blue-500",
			Percent: 0,
		}
	}

	// 4. 数值解析与趋势判断
	currentValue := float64(val.(model.Vector)[0].Value)

	// 这里的 Percent 可以根据你预设的带宽上限（例如 1000Mbps）计算进度条
	percent := int((currentValue / 1000.0) * 100)
	if percent > 100 {
		percent = 100
	}

	return models.StatCard{
		Label:   "实时吞吐",
		Value:   fmt.Sprintf("%.1f", currentValue),
		Unit:    "Mbps",
		Trend:   s.getTrend(wsID, currentValue), // 见下方趋势逻辑
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
	} // 增长超过5%判定为上升
	if current < lastVal.(float64)*0.95 {
		return "down"
	} // 下降超过5%判定为下降
	return "stable"
}

// GetGlobalDashboard 并发查询 VM，聚合全域 Dashboard 数据
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

	// 1. 活跃工作空间数（有数据上报的空间）
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `count(count by (workspace_id) (wireflow_peer_status{workspace_id!=""}))`, time.Now())
		if err == nil && len(vec) > 0 {
			activeWs = int(vec[0].Value)
		}
		return nil
	})

	// 2. 全网在线节点数（按 node_id 去重）
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `count(count by (node_id) (wireflow_peer_status == 1))`, time.Now())
		if err == nil && len(vec) > 0 {
			onlineNodes = int(vec[0].Value)
		}
		return nil
	})

	// 3. 全域总吞吐（Gbps）
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `sum(irate(wireflow_peer_traffic_bytes_total{direction="tx"}[2m])) * 8 / 1e9`, time.Now())
		if err == nil && len(vec) > 0 {
			throughputGbps = float64(vec[0].Value)
		}
		return nil
	})

	// 4. 每个空间的在线节点数
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `count by (workspace_id) (last_over_time(wireflow_node_uptime_seconds[5m]))`, time.Now())
		if err != nil {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		for _, sample := range vec {
			wsID := string(sample.Metric["workspace_id"])
			wsNodes[wsID] = int(sample.Value)
		}
		return nil
	})

	// 5. 每个空间 24h 发送流量
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `sum by (workspace_id) (increase(wireflow_peer_traffic_bytes_total{direction="tx"}[24h]))`, time.Now())
		if err != nil {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		for _, sample := range vec {
			wsID := string(sample.Metric["workspace_id"])
			wsTraffic[wsID] = float64(sample.Value)
		}
		return nil
	})

	// 6. 每个空间健康度（在线 peer 占比）
	eg.Go(func() error {
		vec, err := s.QueryByTime(ctx, `avg by (workspace_id) (wireflow_peer_status) * 100`, time.Now())
		if err != nil {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		for _, sample := range vec {
			wsID := string(sample.Metric["workspace_id"])
			wsHealth[wsID] = float64(sample.Value)
		}
		return nil
	})

	// 7. 全域吞吐趋势（当天 0 点到现在，4h 粒度 → 最多 6 个点）
	eg.Go(func() error {
		midnight := time.Now().Truncate(24 * time.Hour)
		r := v1.Range{
			Start: midnight,
			End:   time.Now(),
			Step:  4 * time.Hour,
		}
		txQ := `sum(irate(wireflow_node_traffic_bytes_total{direction="tx"}[20m])) * 8 / 1e9`
		rxQ := `sum(irate(wireflow_node_traffic_bytes_total{direction="rx"}[20m])) * 8 / 1e9`
		txResult, _, _ := s.api.QueryRange(ctx, txQ, r)
		rxResult, _, _ := s.api.QueryRange(ctx, rxQ, r)
		trend := s.processMatrixToTrendWithRX(txResult, rxResult)
		mu.Lock()
		globalTrend = trend
		mu.Unlock()
		return nil
	})

	// 8. Top 10 节点（24h 流量）+ CPU + 在线状态
	eg.Go(func() error {
		trafficVec, err := s.QueryByTime(ctx,
			`topk(10, sum by (peer_id, network_id) (increase(wireflow_node_traffic_bytes_total[24h])))`,
			time.Now())
		if err != nil {
			return nil
		}

		cpuVec, _ := s.QueryByTime(ctx, `last_over_time(wireflow_node_cpu_usage_percent[5m])`, time.Now())
		cpuMap := make(map[string]float64)
		for _, samp := range cpuVec {
			cpuMap[string(samp.Metric["peer_id"])] = float64(samp.Value)
		}

		statusVec, _ := s.QueryByTime(ctx, `last_over_time(wireflow_peer_status[5m])`, time.Now())
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
				Label: "活跃工作空间", Value: strconv.Itoa(activeWs), Unit: "SETS",
				Trend: "+0", Color: "text-blue-500", BarWidth: calcProgress(activeWs, 20), TrendUp: true,
			},
			{
				Label: "全网在线节点", Value: strconv.Itoa(onlineNodes), Unit: "NODE",
				Trend: "Live", Color: "text-emerald-500", BarWidth: calcProgress(onlineNodes, 2000), TrendUp: true,
			},
			{
				Label: "全域总吞吐", Value: fmt.Sprintf("%.1f", throughputGbps), Unit: "Gbps",
				Trend: "Gbps", Color: "text-primary", BarWidth: calcProgress(int(throughputGbps*10), 100), TrendUp: true,
			},
			{
				Label: "未处理告警", Value: "00", Unit: "WARN",
				Trend: "Healthy", Color: "text-error", BarWidth: "0%", TrendUp: false,
			},
		},
		GlobalEvents: []models.GlobalEventItem{},
	}

	resp.GlobalTrend = globalTrend
	resp.TopNodes = topNodes

	// 合并 workspace 数据（以有流量或有节点的空间为准）
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
			Name:    wsID, // TODO: 后续 join DB 换成 displayName
			Type:    "Production",
			Nodes:   wsNodes[wsID],
			Traffic: formatTrafficBytes(wsTraffic[wsID]),
			Health:  health,
			Status:  status,
		})
	}

	return resp, nil
}

// calcProgress 将 value/max 映射为百分比字符串，如 "65%"
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

// formatTrafficBytes 将字节数格式化为可读字符串
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
