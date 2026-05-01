package controller

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
)

// AlertController defines the alert management operations.
type AlertController interface {
	ListRules(ctx context.Context, wsID string) ([]*models.AlertRule, error)
	GetRule(ctx context.Context, id string) (*models.AlertRule, error)
	CreateRule(ctx context.Context, wsID string, req service.CreateAlertRuleRequest) (*models.AlertRule, error)
	UpdateRule(ctx context.Context, id string, req service.CreateAlertRuleRequest) (*models.AlertRule, error)
	DeleteRule(ctx context.Context, id string) error

	ListHistory(ctx context.Context, wsID string, page, pageSize int) ([]*models.AlertHistory, int64, error)

	ListChannels(ctx context.Context, wsID string) ([]*models.AlertChannel, error)
	CreateChannel(ctx context.Context, wsID string, req service.CreateChannelRequest) (*models.AlertChannel, error)
	UpdateChannel(ctx context.Context, id string, req service.CreateChannelRequest) (*models.AlertChannel, error)
	DeleteChannel(ctx context.Context, id string) error

	ListSilences(ctx context.Context, wsID string) ([]*models.AlertSilence, error)
	CreateSilence(ctx context.Context, wsID, createdBy string, req service.CreateSilenceRequest) (*models.AlertSilence, error)
	DeleteSilence(ctx context.Context, id string) error
}

type alertController struct {
	alertService *service.AlertService
	store        store.Store
}

// NewAlertController creates a new AlertController.
func NewAlertController(st store.Store) AlertController {
	return &alertController{
		alertService: service.NewAlertService(st),
		store:        st,
	}
}

func (c *alertController) ListRules(ctx context.Context, wsID string) ([]*models.AlertRule, error) {
	return c.alertService.ListRules(ctx, wsID)
}

func (c *alertController) GetRule(ctx context.Context, id string) (*models.AlertRule, error) {
	return c.alertService.GetRule(ctx, id)
}

func (c *alertController) CreateRule(ctx context.Context, wsID string, req service.CreateAlertRuleRequest) (*models.AlertRule, error) {
	return c.alertService.CreateRule(ctx, wsID, req)
}

func (c *alertController) UpdateRule(ctx context.Context, id string, req service.CreateAlertRuleRequest) (*models.AlertRule, error) {
	return c.alertService.UpdateRule(ctx, id, req)
}

func (c *alertController) DeleteRule(ctx context.Context, id string) error {
	return c.alertService.DeleteRule(ctx, id)
}

func (c *alertController) ListHistory(ctx context.Context, wsID string, page, pageSize int) ([]*models.AlertHistory, int64, error) {
	return c.alertService.ListHistory(ctx, wsID, page, pageSize)
}

func (c *alertController) ListChannels(ctx context.Context, wsID string) ([]*models.AlertChannel, error) {
	return c.alertService.ListChannels(ctx, wsID)
}

func (c *alertController) CreateChannel(ctx context.Context, wsID string, req service.CreateChannelRequest) (*models.AlertChannel, error) {
	return c.alertService.CreateChannel(ctx, wsID, req)
}

func (c *alertController) UpdateChannel(ctx context.Context, id string, req service.CreateChannelRequest) (*models.AlertChannel, error) {
	return c.alertService.UpdateChannel(ctx, id, req)
}

func (c *alertController) DeleteChannel(ctx context.Context, id string) error {
	return c.alertService.DeleteChannel(ctx, id)
}

func (c *alertController) ListSilences(ctx context.Context, wsID string) ([]*models.AlertSilence, error) {
	return c.alertService.ListSilences(ctx, wsID)
}

func (c *alertController) CreateSilence(ctx context.Context, wsID, createdBy string, req service.CreateSilenceRequest) (*models.AlertSilence, error) {
	return c.alertService.CreateSilence(ctx, wsID, createdBy, req)
}

func (c *alertController) DeleteSilence(ctx context.Context, id string) error {
	return c.alertService.DeleteSilence(ctx, id)
}
