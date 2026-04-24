package controller

import (
	"context"
	"fmt"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/models"
	"wireflow/management/service"
)

type MonitorController interface {
	GetTopologySnapshot(ctx context.Context) ([]models.PeerSnapshot, error)
	GetNodeSnapshot(ctx context.Context) ([]models.NodeSnapshot, error)
	GetWorkspaceAggregatedMonitor(ctx context.Context, wsID string) (*models.AggregatedMonitorResponse, error)
	// GetWorkspaceDashboard returns workspace-scoped dashboard data.
	// wsID is the workspace UUID; the controller resolves it to a namespace for PromQL.
	GetWorkspaceDashboard(ctx context.Context, wsID string) (*models.WorkspaceDashboardResponse, error)
	GetGlobalDashboard(ctx context.Context) (*models.DashboardResponse, error)
}

type monitorController struct {
	monitorService service.MonitorService
	store          store.Store
	log            *log.Logger
}

func (m *monitorController) GetWorkspaceAggregatedMonitor(ctx context.Context, wsID string) (*models.AggregatedMonitorResponse, error) {
	ns, err := m.resolveNamespace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	return m.monitorService.GetWorkspaceAggregatedMonitor(ctx, ns)
}

func (m *monitorController) GetWorkspaceDashboard(ctx context.Context, wsID string) (*models.WorkspaceDashboardResponse, error) {
	ns, err := m.resolveNamespace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	return m.monitorService.GetWorkspaceDashboard(ctx, ns)
}

func (m *monitorController) GetGlobalDashboard(ctx context.Context) (*models.DashboardResponse, error) {
	return m.monitorService.GetGlobalDashboard(ctx)
}

func (m *monitorController) GetNodeSnapshot(ctx context.Context) ([]models.NodeSnapshot, error) {
	// workspace_id must come from the request context in production;
	// returning empty here until callers pass the workspace context.
	return nil, nil
}

func (m *monitorController) GetTopologySnapshot(ctx context.Context) ([]models.PeerSnapshot, error) {
	return m.monitorService.GetTopologySnapshot(ctx)
}

// resolveNamespace looks up the workspace.Namespace (= network_id in metrics) for the given wsID.
func (m *monitorController) resolveNamespace(ctx context.Context, wsID string) (string, error) {
	ws, err := m.store.Workspaces().GetByID(ctx, wsID)
	if err != nil {
		return "", fmt.Errorf("workspace %s not found: %w", wsID, err)
	}
	if ws.Namespace == "" {
		return "", fmt.Errorf("workspace %s has no namespace configured", wsID)
	}
	return ws.Namespace, nil
}

// ── noop implementation (used when monitor address is empty / VM unreachable) ──

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
func (n *noopMonitorController) GetWorkspaceDashboard(_ context.Context, _ string) (*models.WorkspaceDashboardResponse, error) {
	return &models.WorkspaceDashboardResponse{
		StatCards:       []models.WorkspaceStatCard{},
		ThroughputTrend: models.TrendData{},
		NodeCPU:         []models.NodeCPUItem{},
		TopNodes:        []models.NodeMonitorDetail{},
	}, nil
}
func (n *noopMonitorController) GetGlobalDashboard(_ context.Context) (*models.DashboardResponse, error) {
	return &models.DashboardResponse{
		GlobalStats:    []models.GlobalStatItem{},
		WorkspaceUsage: []models.WorkspaceUsageRow{},
		GlobalEvents:   []models.GlobalEventItem{},
	}, nil
}

func NewMonitorController(address string, st store.Store) MonitorController {
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
		store:          st,
		log:            logger,
	}
}
