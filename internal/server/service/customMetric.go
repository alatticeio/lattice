package service

import (
	"context"
	"fmt"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/google/uuid"
)

// CustomMetricService handles custom metric CRUD operations.
type CustomMetricService struct {
	store  store.Store
	logger *log.Logger
}

// NewCustomMetricService creates a new CustomMetricService.
func NewCustomMetricService(st store.Store) *CustomMetricService {
	return &CustomMetricService{store: st, logger: log.GetLogger("custom-metric-service")}
}

// CreateCustomMetricRequest is the request body for creating/updating a custom metric.
type CreateCustomMetricRequest struct {
	Name       string `json:"name"`
	Query      string `json:"query"`
	Type       string `json:"type"`
	ResultType string `json:"result_type"`
	Labels     string `json:"labels"`
}

// Create creates a new custom metric.
func (s *CustomMetricService) Create(ctx context.Context, wsID, createdBy string, req CreateCustomMetricRequest) (*models.CustomMetric, error) {
	m := &models.CustomMetric{
		Model:       models.Model{ID: uuid.New().String()},
		Name:        req.Name,
		WorkspaceID: wsID,
		Query:       req.Query,
		Type:        req.Type,
		ResultType:  req.ResultType,
		Labels:      req.Labels,
		CreatedBy:   createdBy,
	}
	if err := s.store.CustomMetrics().Create(ctx, m); err != nil {
		return nil, fmt.Errorf("create custom metric: %w", err)
	}
	return m, nil
}

// List lists all custom metrics for a workspace.
func (s *CustomMetricService) List(ctx context.Context, wsID string) ([]*models.CustomMetric, error) {
	return s.store.CustomMetrics().ListByWorkspace(ctx, wsID)
}

// Get gets a single custom metric by ID.
func (s *CustomMetricService) Get(ctx context.Context, id string) (*models.CustomMetric, error) {
	return s.store.CustomMetrics().GetByID(ctx, id)
}

// Update updates an existing custom metric.
func (s *CustomMetricService) Update(ctx context.Context, id string, req CreateCustomMetricRequest) (*models.CustomMetric, error) {
	m, err := s.store.CustomMetrics().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Name = req.Name
	m.Query = req.Query
	m.Type = req.Type
	m.ResultType = req.ResultType
	m.Labels = req.Labels
	if err := s.store.CustomMetrics().Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Delete deletes a custom metric by ID.
func (s *CustomMetricService) Delete(ctx context.Context, id string) error {
	return s.store.CustomMetrics().Delete(ctx, id)
}
