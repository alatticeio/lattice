package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"wireflow/internal/log"
	"wireflow/management/models"
	"wireflow/monitor"
	"wireflow/pkg/utils"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup"
)

type MonitorService interface {
	GetTopologySnapshot(ctx context.Context) ([]monitor.PeerSnapshot, error)
	GetNodeSnapshot(ctx context.Context, wsID string) ([]models.NodeSnapshot, error)
	GetWorkspaceAggregatedMonitor(ctx context.Context, wsID string) (*models.AggregatedMonitorResponse, error)
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
func (v *monitorService) GetTopologySnapshot(ctx context.Context) ([]monitor.PeerSnapshot, error) {
	// 1. 查询所有以 wireflow_node_ 开头的指标
	query := `last_over_time({__name__=~"wireflow_node_.*"}[5m])`
	vector, err := v.QueryByTime(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	nodeMap := make(map[string]*monitor.PeerSnapshot)

	for _, s := range vector {
		nodeID := string(s.Metric["node_id"])
		metricName := string(s.Metric["__name__"])
		val := float64(s.Value)

		// 初始化节点
		if _, ok := nodeMap[nodeID]; !ok {
			nodeMap[nodeID] = &monitor.PeerSnapshot{
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
	var result []monitor.PeerSnapshot
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
		// 查询最近 1 分钟的平均吞吐量
		//query := fmt.Sprintf(`sum(irate(wireflow_interface_tx_bytes_total{ws_id="%s"}[1m])) * 8 / 1000000`, wsID)
		//val, _, err := s.api.Query(ctx, query, time.Now())
		//if err == nil {
		//	resp.LiveStats[0] = models.StatCard{
		//		Label: "实时吞吐",
		//		Value: s.formatVectorValue(val),
		//		Unit:  "Mbps",
		//		Color: "text-blue-500",
		//		Trend: "up",
		//	}
		//}
		//return err
		resp.LiveStats[0] = s.fetchThroughput(ctx, wsID)
		return nil
	})

	// 2. 获取平均延迟
	eg.Go(func() error {
		query := fmt.Sprintf(`avg(wireflow_node_latency_ms{ws_id="%s"})`, wsID)
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

	// 2. 获取丢包率
	eg.Go(func() error {
		query := fmt.Sprintf(`avg(wireflow_node_latency_ms{ws_id="%s"})`, wsID)
		val, _, err := s.api.Query(ctx, query, time.Now())
		if err == nil {
			resp.LiveStats[2] = models.StatCard{
				Label: "丢包率",
				Value: s.formatVectorValue(val),
				Unit:  "ms",
				Color: "text-emerald-500",
			}
		}
		return err
	})

	// 2. 获取平均延迟
	eg.Go(func() error {
		query := fmt.Sprintf(`avg(wireflow_node_latency_ms{ws_id="%s"})`, wsID)
		val, _, err := s.api.Query(ctx, query, time.Now())
		if err == nil {
			resp.LiveStats[3] = models.StatCard{
				Label: "活动隧道",
				Value: s.formatVectorValue(val),
				Unit:  "uint",
				Color: "text-emerald-500",
			}
		}
		return err
	})

	// 3. 获取趋势图数据 (过去 1 小时)
	eg.Go(func() error {
		r := v1.Range{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now(),
			Step:  time.Minute * 2,
		}
		query := fmt.Sprintf(`sum(irate(wireflow_interface_tx_bytes_total{ws_id="%s"}[5m]))`, wsID)
		result, _, err := s.api.QueryRange(ctx, query, r)
		if err == nil {
			resp.Trend = s.processMatrixToTrend(result)
		}
		return err
	})

	// 4. 获取节点列表明细 (通过 VM 里的最新样本判定)
	eg.Go(func() error {
		// 获取各节点的最新流量和模式标签
		query := fmt.Sprintf(`last_over_time(wireflow_node_info{ws_id="%s"}[5m])`, wsID)
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
func (s *monitorService) processMatrixToTrend(val model.Value) models.TrendData {
	matrix, ok := val.(model.Matrix)
	if !ok || len(matrix) == 0 {
		return models.TrendData{}
	}

	trend := models.TrendData{}
	for _, sample := range matrix[0].Values {
		trend.Timestamps = append(trend.Timestamps, sample.Timestamp.Time().Format("15:04"))
		trend.TXData = append(trend.TXData, float64(sample.Value))
	}
	return trend
}

// 将节点标签信息转换为明细列表
func (s *monitorService) convertVectorToNodes(val model.Value) []models.NodeMonitorDetail {
	vector, _ := val.(model.Vector)
	nodes := make([]models.NodeMonitorDetail, 0)
	for _, sample := range vector {
		nodes = append(nodes, models.NodeMonitorDetail{
			Name:           string(sample.Metric["node_name"]),
			VIP:            string(sample.Metric["vip"]),
			ConnectionType: string(sample.Metric["conn_type"]), // p2p 或 relay
			Online:         true,                               // 如果能查到最近 5 分钟的样本则视为在线
		})
	}
	return nodes
}

func (s *monitorService) fetchThroughput(ctx context.Context, wsID string) models.StatCard {
	// 1. 定义查询语句
	// 使用 irate 获取瞬时速率
	query := fmt.Sprintf(`sum(irate(wireflow_interface_tx_bytes_total{ws_id="%s"}[1m])) * 8 / 1000000`, wsID)

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
