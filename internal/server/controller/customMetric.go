package controller

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
)

// CustomMetricController defines the custom metric operations.
type CustomMetricController interface {
	List(ctx context.Context, wsID string) ([]*models.CustomMetric, error)
	Get(ctx context.Context, id string) (*models.CustomMetric, error)
	Create(ctx context.Context, wsID, createdBy string, req service.CreateCustomMetricRequest) (*models.CustomMetric, error)
	Update(ctx context.Context, id string, req service.CreateCustomMetricRequest) (*models.CustomMetric, error)
	Delete(ctx context.Context, id string) error
}

type customMetricController struct {
	svc   *service.CustomMetricService
	store store.Store
}

// NewCustomMetricController creates a new CustomMetricController.
func NewCustomMetricController(st store.Store) CustomMetricController {
	return &customMetricController{
		svc:   service.NewCustomMetricService(st),
		store: st,
	}
}

func (c *customMetricController) List(ctx context.Context, wsID string) ([]*models.CustomMetric, error) {
	return c.svc.List(ctx, wsID)
}

func (c *customMetricController) Get(ctx context.Context, id string) (*models.CustomMetric, error) {
	return c.svc.Get(ctx, id)
}

func (c *customMetricController) Create(ctx context.Context, wsID, createdBy string, req service.CreateCustomMetricRequest) (*models.CustomMetric, error) {
	return c.svc.Create(ctx, wsID, createdBy, req)
}

func (c *customMetricController) Update(ctx context.Context, id string, req service.CreateCustomMetricRequest) (*models.CustomMetric, error) {
	return c.svc.Update(ctx, id, req)
}

func (c *customMetricController) Delete(ctx context.Context, id string) error {
	return c.svc.Delete(ctx, id)
}
