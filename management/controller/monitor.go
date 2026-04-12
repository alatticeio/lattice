package controller

import (
	"context"
	"errors"
	"wireflow/internal/log"
	"wireflow/management/models"
	"wireflow/management/service"
)

type MonitorController interface {
	GetTopologySnapshot(ctx context.Context) ([]models.PeerSnapshot, error)
	GetNodeSnapshot(ctx context.Context) ([]models.NodeSnapshot, error)
	GetWorkspaceAggregatedMonitor(ctx context.Context, wsID string) (*models.AggregatedMonitorResponse, error)
	GetGlobalDashboard(ctx context.Context) (*models.DashboardResponse, error)
}

type monitorController struct {
	monitorService service.MonitorService
	log            *log.Logger
}

func (m *monitorController) GetWorkspaceAggregatedMonitor(ctx context.Context, wsID string) (*models.AggregatedMonitorResponse, error) {
	return m.monitorService.GetWorkspaceAggregatedMonitor(ctx, wsID)
}

func (m *monitorController) GetGlobalDashboard(ctx context.Context) (*models.DashboardResponse, error) {
	return m.monitorService.GetGlobalDashboard(ctx)
}

func (m *monitorController) GetNodeSnapshot(ctx context.Context) ([]models.NodeSnapshot, error) {
	//wsID := ctx.Value("workspace_id").(string)
	wsID := "22aec1e6-e7f4-4079-b65e-c12ecdd57d60"
	if wsID == "" {
		return nil, errors.New("workspace_id is empty")
	}
	return m.monitorService.GetNodeSnapshot(ctx, wsID)
}

func (m *monitorController) GetTopologySnapshot(ctx context.Context) ([]models.PeerSnapshot, error) {
	return m.monitorService.GetTopologySnapshot(ctx)
}

// noopMonitorController 是 Monitor 不可用时的降级实现，所有查询返回空结果。
type noopMonitorController struct{}

func (n *noopMonitorController) GetTopologySnapshot(_ context.Context) ([]models.PeerSnapshot, error) {
	return nil, nil
}
func (n *noopMonitorController) GetNodeSnapshot(_ context.Context) ([]models.NodeSnapshot, error) {
	return nil, nil
}
func (n *noopMonitorController) GetWorkspaceAggregatedMonitor(_ context.Context, _ string) (*models.AggregatedMonitorResponse, error) {
	return nil, nil
}

func (n *noopMonitorController) GetGlobalDashboard(_ context.Context) (*models.DashboardResponse, error) {
	return &models.DashboardResponse{
		GlobalStats:    []models.GlobalStatItem{},
		WorkspaceUsage: []models.WorkspaceUsageRow{},
		GlobalEvents:   []models.GlobalEventItem{},
	}, nil
}

func NewMonitorController(address string) MonitorController {
	logger := log.GetLogger("monitor-controller")
	if address == "" {
		logger.Warn("monitor address is empty, monitor controller disabled (noop)")
		return &noopMonitorController{}
	}
	svc, err := service.NewMonitorService(address)
	if err != nil {
		logger.Warn("init monitor service failed, falling back to noop", "err", err)
		return &noopMonitorController{}
	}
	return &monitorController{
		monitorService: svc,
		log:            logger,
	}
}
