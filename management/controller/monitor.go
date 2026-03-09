package controller

import (
	"context"
	"errors"
	"wireflow/internal/log"
	"wireflow/management/models"
	"wireflow/management/service"
	"wireflow/monitor"
)

type MonitorController interface {
	GetTopologySnapshot(ctx context.Context) ([]monitor.PeerSnapshot, error)
	GetNodeSnapshot(ctx context.Context) ([]models.NodeSnapshot, error)
	GetWorkspaceAggregatedMonitor(ctx context.Context, wsID string) (*models.AggregatedMonitorResponse, error)
}

type monitorController struct {
	monitorService service.MonitorService
	log            *log.Logger
}

func (m *monitorController) GetWorkspaceAggregatedMonitor(ctx context.Context, wsID string) (*models.AggregatedMonitorResponse, error) {
	return m.monitorService.GetWorkspaceAggregatedMonitor(ctx, wsID)
}

func (m *monitorController) GetNodeSnapshot(ctx context.Context) ([]models.NodeSnapshot, error) {
	//wsID := ctx.Value("workspace_id").(string)
	wsID := "22aec1e6-e7f4-4079-b65e-c12ecdd57d60"
	if wsID == "" {
		return nil, errors.New("workspace_id is empty")
	}
	return m.monitorService.GetNodeSnapshot(ctx, wsID)
}

func (m *monitorController) GetTopologySnapshot(ctx context.Context) ([]monitor.PeerSnapshot, error) {
	return m.monitorService.GetTopologySnapshot(ctx)
}

func NewMonitorController(address string) MonitorController {
	logger := log.GetLogger("monitor-controller")
	svc, err := service.NewMonitorService(address)
	if err != nil {
		logger.Error("init monitor service failed", err)
	}
	return &monitorController{
		monitorService: svc,
		log:            logger,
	}
}
