package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/monitor/adapter"
	"github.com/alatticeio/lattice/internal/monitor/alert/notifier"
	"github.com/alatticeio/lattice/internal/monitor/gateway"
	"github.com/alatticeio/lattice/internal/server/models"
)

// ActiveAlert tracks an alert that is currently in "firing" state.
type ActiveAlert struct {
	RuleID    string
	GroupKey  string
	FirstSeen time.Time
	LastSeen  time.Time
	Value     float64
	Labels    map[string]string
}

// AlertEngine evaluates alert rules on a periodic ticker and dispatches
// notifications when thresholds are breached or resolved.
type AlertEngine struct {
	gateway      *gateway.MonitorGateway
	store        store.Store
	notifiers    map[string]notifier.Notifier
	activeAlerts map[string]*ActiveAlert
	mu           sync.RWMutex
	logger       *log.Logger
	evalInterval time.Duration
}

// NewEngine creates a new AlertEngine.
func NewEngine(gw *gateway.MonitorGateway, st store.Store, notifiers map[string]notifier.Notifier) *AlertEngine {
	return &AlertEngine{
		gateway:      gw,
		store:        st,
		notifiers:    notifiers,
		activeAlerts: make(map[string]*ActiveAlert),
		logger:       log.GetLogger("alert-engine"),
		evalInterval: 30 * time.Second,
	}
}

// Start runs the evaluation loop until ctx is cancelled.
func (e *AlertEngine) Start(ctx context.Context) {
	ticker := time.NewTicker(e.evalInterval)
	defer ticker.Stop()

	e.logger.Info("alert engine started", "interval", e.evalInterval)
	e.evaluate(ctx)

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("alert engine stopped")
			return
		case <-ticker.C:
			e.evaluate(ctx)
		}
	}
}

func (e *AlertEngine) evaluate(ctx context.Context) {
	rules, err := e.store.Alerts().ListEnabledAlertRules(ctx)
	if err != nil {
		e.logger.Error("failed to load alert rules", err)
		return
	}
	for _, rule := range rules {
		if err := e.evaluateRule(ctx, rule); err != nil {
			e.logger.Warn("rule evaluation failed", "rule_id", rule.ID, "err", err)
		}
	}
}

func (e *AlertEngine) evaluateRule(ctx context.Context, rule *models.AlertRule) error {
	if e.isSilenced(ctx, rule.WorkspaceID, rule) {
		return nil
	}

	req := &adapter.QueryRequest{
		MetricType: rule.MetricType,
		Labels:     map[string]string{},
		Namespace:  rule.WorkspaceID,
		TimeRange:  adapter.TimeRange{End: time.Now(), Lookback: rule.Lookback},
	}

	result, err := e.gateway.Query(ctx, req)
	if err != nil {
		return fmt.Errorf("query metric: %w", err)
	}
	if result.Scalar == nil {
		return nil
	}

	currentValue := result.Scalar.Value
	thresholdMet := compareThreshold(currentValue, rule.Operator, rule.Threshold)
	alertKey := rule.ID + ":default"

	if thresholdMet {
		e.handleFiring(ctx, rule, alertKey, currentValue)
	} else {
		e.handleResolved(ctx, rule, alertKey, currentValue)
	}
	return nil
}

// compareThreshold checks if a value satisfies the operator/threshold condition.
func compareThreshold(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	case "neq":
		return value != threshold
	default:
		return false
	}
}

func (e *AlertEngine) handleFiring(ctx context.Context, rule *models.AlertRule, key string, value float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	active, exists := e.activeAlerts[key]
	now := time.Now()

	if !exists {
		e.activeAlerts[key] = &ActiveAlert{
			RuleID:    rule.ID,
			GroupKey:  key,
			FirstSeen: now,
			LastSeen:  now,
			Value:     value,
		}

		history := &models.AlertHistory{
			RuleID:      rule.ID,
			WorkspaceID: rule.WorkspaceID,
			Status:      "firing",
			Severity:    rule.Severity,
			Value:       value,
			StartedAt:   now,
			Message:     rule.Message,
		}
		if err := e.store.Alerts().CreateAlertHistory(ctx, history); err != nil {
			e.logger.Error("failed to create alert history", err)
			return
		}

		e.sendNotification(ctx, rule, []notifier.AlertItem{
			{Labels: map[string]string{}, Value: value, Message: rule.Message},
		}, "firing")
	} else {
		active.LastSeen = now
		active.Value = value
	}
}

func (e *AlertEngine) handleResolved(ctx context.Context, rule *models.AlertRule, key string, value float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	active, exists := e.activeAlerts[key]
	if !exists {
		return
	}
	delete(e.activeAlerts, key)

	ended := time.Now()
	history := &models.AlertHistory{
		RuleID:      rule.ID,
		WorkspaceID: rule.WorkspaceID,
		Status:      "resolved",
		Severity:    rule.Severity,
		Value:       value,
		StartedAt:   active.FirstSeen,
		EndedAt:     &ended,
		Message:     fmt.Sprintf("Resolved: %s", rule.Message),
	}
	if err := e.store.Alerts().CreateAlertHistory(ctx, history); err != nil {
		e.logger.Error("failed to update alert history", err)
		return
	}

	e.sendNotification(ctx, rule, []notifier.AlertItem{
		{Labels: map[string]string{}, Value: value, Message: history.Message},
	}, "resolved")
}

func (e *AlertEngine) sendNotification(ctx context.Context, rule *models.AlertRule, items []notifier.AlertItem, status string) {
	var channelIDs []string
	if rule.Channels != "" {
		if err := json.Unmarshal([]byte(rule.Channels), &channelIDs); err != nil {
			e.logger.Warn("failed to parse channel IDs", "err", err)
			return
		}
	}

	req := &notifier.NotificationRequest{
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		Status:    status,
		Alerts:    items,
		StartedAt: time.Now(),
	}

	for _, chID := range channelIDs {
		ch, err := e.store.Alerts().GetAlertChannel(ctx, chID)
		if err != nil {
			e.logger.Warn("alert channel not found", "id", chID)
			continue
		}
		if !ch.Enabled {
			continue
		}
		n, ok := e.notifiers[ch.Type]
		if !ok {
			e.logger.Warn("unknown notifier type", "type", ch.Type)
			continue
		}
		if err := n.Send(ctx, req, []byte(ch.Config)); err != nil {
			e.logger.Error("notification send failed", err, "notifier", ch.Type)
		}
	}
}

func (e *AlertEngine) isSilenced(ctx context.Context, wsID string, rule *models.AlertRule) bool {
	if rule.SilenceUntil != nil && time.Now().Before(*rule.SilenceUntil) {
		return true
	}
	silences, err := e.store.Alerts().ListAlertSilences(ctx, wsID)
	if err != nil {
		return false
	}
	now := time.Now()
	for _, s := range silences {
		if now.After(s.StartsAt) && now.Before(s.EndsAt) {
			return true
		}
	}
	return false
}
