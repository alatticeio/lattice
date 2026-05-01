package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DingTalkConfig holds the configuration for a DingTalk webhook.
type DingTalkConfig struct {
	WebhookURL string   `json:"webhook_url"`
	Secret     string   `json:"secret"`
	AtMobiles  []string `json:"at_mobiles"`
}

// DingTalkNotifier sends alerts to DingTalk group chat.
type DingTalkNotifier struct{ client *http.Client }

// NewDingTalkNotifier creates a new DingTalk notifier.
func NewDingTalkNotifier() *DingTalkNotifier {
	return &DingTalkNotifier{client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *DingTalkNotifier) Type() string { return "dingtalk" }

func (n *DingTalkNotifier) Send(ctx context.Context, req *NotificationRequest, configBytes []byte) error {
	var cfg DingTalkConfig
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return fmt.Errorf("parse dingtalk config: %w", err)
	}
	if cfg.WebhookURL == "" {
		return fmt.Errorf("dingtalk webhook URL is empty")
	}

	targetURL := cfg.WebhookURL
	if cfg.Secret != "" {
		ts := time.Now().UnixMilli()
		sign := signDingTalk(cfg.Secret, ts)
		targetURL += fmt.Sprintf("&timestamp=%d&sign=%s", ts, url.QueryEscape(sign))
	}

	msg := buildDingTalkMessage(req)
	body, _ := json.Marshal(msg)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("dingtalk request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("dingtalk returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func signDingTalk(secret string, timestamp int64) string {
	str := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(str))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func buildDingTalkMessage(req *NotificationRequest) map[string]any {
	statusIcon := "🔴"
	if req.Status == "resolved" {
		statusIcon = "🟢"
	}
	content := fmt.Sprintf("%s [%s] %s\nStatus: %s\nSeverity: %s",
		statusIcon, req.Severity, req.RuleName, req.Status, req.Severity)
	for _, a := range req.Alerts {
		content += fmt.Sprintf("\nValue: %.2f", a.Value)
		if a.Message != "" {
			content += fmt.Sprintf("\n%s", a.Message)
		}
	}
	msg := map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": content},
	}
	if len(req.Alerts) > 0 && req.Alerts[0].Labels != nil {
		msg["at"] = map[string]any{"atMobiles": []string{}}
	}
	return msg
}
