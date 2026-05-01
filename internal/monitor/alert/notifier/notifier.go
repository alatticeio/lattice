package notifier

import (
	"context"
	"time"
)

// AlertItem represents a single alert in a notification.
type AlertItem struct {
	Labels  map[string]string
	Value   float64
	Message string
}

// NotificationRequest contains the data needed to send a notification.
type NotificationRequest struct {
	RuleName  string
	Severity  string
	Status    string // "firing" or "resolved"
	Alerts    []AlertItem
	StartedAt time.Time
}

// Notifier defines the interface for sending alert notifications.
type Notifier interface {
	// Type returns the notifier type identifier (e.g., "webhook", "slack").
	Type() string
	// Send dispatches a notification to a specific channel.
	Send(ctx context.Context, req *NotificationRequest, config []byte) error
}
