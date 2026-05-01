package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookNotifier_Send(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := WebhookConfig{
		URL:    ts.URL,
		Method: http.MethodPost,
		Headers: map[string]string{
			"Authorization": "Bearer secret-token",
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	n := NewWebhookNotifier()
	req := &NotificationRequest{
		RuleName:  "test-rule",
		Severity:  "critical",
		Status:    "firing",
		StartedAt: time.Now(),
		Alerts:    []AlertItem{{Value: 95.5, Message: "High CPU"}},
	}

	err = n.Send(context.Background(), req, cfgBytes)
	assert.NoError(t, err)
}

func TestWebhookNotifier_SendServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := WebhookConfig{URL: ts.URL}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	n := NewWebhookNotifier()
	req := &NotificationRequest{RuleName: "test", Severity: "critical", Status: "firing"}

	err = n.Send(context.Background(), req, cfgBytes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestWebhookNotifier_Type(t *testing.T) {
	n := NewWebhookNotifier()
	assert.Equal(t, "webhook", n.Type())
}

func TestWebhookNotifier_EmptyURL(t *testing.T) {
	n := NewWebhookNotifier()
	cfgBytes := []byte(`{}`)
	req := &NotificationRequest{RuleName: "test"}

	err := n.Send(context.Background(), req, cfgBytes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestWebhookNotifier_InvalidConfig(t *testing.T) {
	n := NewWebhookNotifier()
	req := &NotificationRequest{RuleName: "test"}

	err := n.Send(context.Background(), req, []byte(`not json`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse webhook config")
}
