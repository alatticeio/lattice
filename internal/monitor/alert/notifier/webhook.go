package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookConfig holds the configuration for a generic webhook.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

// WebhookNotifier sends alerts to a generic HTTP webhook endpoint.
type WebhookNotifier struct{ client *http.Client }

// NewWebhookNotifier creates a new webhook notifier.
func NewWebhookNotifier() *WebhookNotifier {
	return &WebhookNotifier{client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *WebhookNotifier) Type() string { return "webhook" }

func (n *WebhookNotifier) Send(ctx context.Context, req *NotificationRequest, configBytes []byte) error {
	var cfg WebhookConfig
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return fmt.Errorf("parse webhook config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("webhook URL is empty")
	}
	method := cfg.Method
	if method == "" {
		method = http.MethodPost
	}

	payload := map[string]any{
		"rule_name":  req.RuleName,
		"severity":   req.Severity,
		"status":     req.Status,
		"started_at": req.StartedAt.Format(time.RFC3339),
		"alerts":     req.Alerts,
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, method, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := n.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
