package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/google/uuid"
)

// AlertService handles alert rule, channel, silence, and history operations.
type AlertService struct {
	store  store.Store
	logger *log.Logger
}

// NewAlertService creates a new AlertService.
func NewAlertService(st store.Store) *AlertService {
	return &AlertService{store: st, logger: log.GetLogger("alert-service")}
}

// CreateAlertRuleRequest is the request body for creating/updating an alert rule.
type CreateAlertRuleRequest struct {
	Name       string   `json:"name"`
	MetricType string   `json:"metric_type"`
	Operator   string   `json:"operator"`
	Threshold  float64  `json:"threshold"`
	Duration   string   `json:"duration"`
	Lookback   string   `json:"lookback"`
	GroupBy    []string `json:"group_by"`
	ForEach    bool     `json:"for_each"`
	Channels   []string `json:"channels"`
	Severity   string   `json:"severity"`
	Message    string   `json:"message"`
}

// CreateRule creates a new alert rule.
func (s *AlertService) CreateRule(ctx context.Context, wsID string, req CreateAlertRuleRequest) (*models.AlertRule, error) {
	groupBy, _ := json.Marshal(req.GroupBy)
	channels, _ := json.Marshal(req.Channels)

	rule := &models.AlertRule{
		Model:       models.Model{ID: uuid.New().String()},
		Name:        req.Name,
		WorkspaceID: wsID,
		Enabled:     true,
		MetricType:  req.MetricType,
		Operator:    req.Operator,
		Threshold:   req.Threshold,
		Duration:    req.Duration,
		Lookback:    req.Lookback,
		GroupBy:     string(groupBy),
		ForEach:     req.ForEach,
		Channels:    string(channels),
		Severity:    req.Severity,
		Message:     req.Message,
	}

	if err := s.store.Alerts().CreateAlertRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("create alert rule: %w", err)
	}
	return rule, nil
}

// ListRules lists all alert rules for a workspace.
func (s *AlertService) ListRules(ctx context.Context, wsID string) ([]*models.AlertRule, error) {
	return s.store.Alerts().ListAlertRulesByWorkspace(ctx, wsID)
}

// GetRule gets a single alert rule by ID.
func (s *AlertService) GetRule(ctx context.Context, id string) (*models.AlertRule, error) {
	return s.store.Alerts().GetAlertRule(ctx, id)
}

// UpdateRule updates an existing alert rule.
func (s *AlertService) UpdateRule(ctx context.Context, id string, req CreateAlertRuleRequest) (*models.AlertRule, error) {
	rule, err := s.store.Alerts().GetAlertRule(ctx, id)
	if err != nil {
		return nil, err
	}
	groupBy, _ := json.Marshal(req.GroupBy)
	channels, _ := json.Marshal(req.Channels)

	rule.Name = req.Name
	rule.MetricType = req.MetricType
	rule.Operator = req.Operator
	rule.Threshold = req.Threshold
	rule.Duration = req.Duration
	rule.Lookback = req.Lookback
	rule.GroupBy = string(groupBy)
	rule.ForEach = req.ForEach
	rule.Channels = string(channels)
	rule.Severity = req.Severity
	rule.Message = req.Message

	if err := s.store.Alerts().UpdateAlertRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// DeleteRule deletes an alert rule by ID.
func (s *AlertService) DeleteRule(ctx context.Context, id string) error {
	return s.store.Alerts().DeleteAlertRule(ctx, id)
}

// ListHistory lists alert history for a workspace with pagination.
func (s *AlertService) ListHistory(ctx context.Context, wsID string, page, pageSize int) ([]*models.AlertHistory, int64, error) {
	return s.store.Alerts().ListAlertHistory(ctx, wsID, page, pageSize)
}

// CreateChannelRequest is the request body for creating/updating an alert channel.
type CreateChannelRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Config any    `json:"config"`
}

// CreateChannel creates a new alert channel.
func (s *AlertService) CreateChannel(ctx context.Context, wsID string, req CreateChannelRequest) (*models.AlertChannel, error) {
	cfgBytes, _ := json.Marshal(req.Config)
	ch := &models.AlertChannel{
		Model:       models.Model{ID: uuid.New().String()},
		Name:        req.Name,
		WorkspaceID: wsID,
		Type:        req.Type,
		Config:      string(cfgBytes),
		Enabled:     true,
	}
	if err := s.store.Alerts().CreateAlertChannel(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

// ListChannels lists all alert channels for a workspace.
func (s *AlertService) ListChannels(ctx context.Context, wsID string) ([]*models.AlertChannel, error) {
	return s.store.Alerts().ListAlertChannels(ctx, wsID)
}

// UpdateChannel updates an existing alert channel.
func (s *AlertService) UpdateChannel(ctx context.Context, id string, req CreateChannelRequest) (*models.AlertChannel, error) {
	ch, err := s.store.Alerts().GetAlertChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	cfgBytes, _ := json.Marshal(req.Config)
	ch.Name = req.Name
	ch.Type = req.Type
	ch.Config = string(cfgBytes)
	if err := s.store.Alerts().UpdateAlertChannel(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

// DeleteChannel deletes an alert channel by ID.
func (s *AlertService) DeleteChannel(ctx context.Context, id string) error {
	return s.store.Alerts().DeleteAlertChannel(ctx, id)
}

// CreateSilenceRequest is the request body for creating an alert silence.
type CreateSilenceRequest struct {
	Matchers []map[string]string `json:"matchers"`
	Comment  string              `json:"comment"`
	StartsAt string              `json:"starts_at"`
	EndsAt   string              `json:"ends_at"`
}

// CreateSilence creates a new alert silence.
func (s *AlertService) CreateSilence(ctx context.Context, wsID, createdBy string, req CreateSilenceRequest) (*models.AlertSilence, error) {
	matchers, _ := json.Marshal(req.Matchers)
	startsAt, _ := time.Parse(time.RFC3339, req.StartsAt)
	endsAt, _ := time.Parse(time.RFC3339, req.EndsAt)

	silence := &models.AlertSilence{
		Model:       models.Model{ID: uuid.New().String()},
		WorkspaceID: wsID,
		CreatedBy:   createdBy,
		Matchers:    string(matchers),
		Comment:     req.Comment,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
	}
	if err := s.store.Alerts().CreateAlertSilence(ctx, silence); err != nil {
		return nil, err
	}
	return silence, nil
}

// ListSilences lists all alert silences for a workspace.
func (s *AlertService) ListSilences(ctx context.Context, wsID string) ([]*models.AlertSilence, error) {
	return s.store.Alerts().ListAlertSilences(ctx, wsID)
}

// DeleteSilence deletes an alert silence by ID.
func (s *AlertService) DeleteSilence(ctx context.Context, id string) error {
	return s.store.Alerts().DeleteAlertSilence(ctx, id)
}
