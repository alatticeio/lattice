package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackConfig holds the configuration for a Slack webhook.
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel"`
	IconEmoji  string `json:"icon_emoji"`
}

// SlackNotifier sends alerts to a Slack channel via Incoming Webhook.
type SlackNotifier struct{ client *http.Client }

// NewSlackNotifier creates a new Slack notifier.
func NewSlackNotifier() *SlackNotifier {
	return &SlackNotifier{client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *SlackNotifier) Type() string { return "slack" }

func (n *SlackNotifier) Send(ctx context.Context, req *NotificationRequest, configBytes []byte) error {
	var cfg SlackConfig
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return fmt.Errorf("parse slack config: %w", err)
	}
	if cfg.WebhookURL == "" {
		return fmt.Errorf("slack webhook URL is empty")
	}

	color := "#ff0000"
	if req.Status == "resolved" {
		color = "#36a64f"
	} else if req.Severity == "warning" {
		color = "#ffaa00"
	}

	text := fmt.Sprintf("*[%s]* %s — *%s*\n", req.Severity, req.RuleName, req.Status)
	for _, a := range req.Alerts {
		text += fmt.Sprintf("Value: `%.2f`", a.Value)
		if a.Message != "" {
			text += fmt.Sprintf(" | %s", a.Message)
		}
		text += "\n"
	}

	payload := map[string]any{
		"attachments": []map[string]any{{
			"color":  color,
			"title":  fmt.Sprintf("[%s] %s - %s", req.Severity, req.RuleName, req.Status),
			"text":   text,
			"footer": "Lattice Alert",
			"ts":     req.StartedAt.Unix(),
		}},
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("slack request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	return nil
}
