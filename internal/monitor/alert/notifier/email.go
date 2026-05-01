package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// EmailConfig holds the configuration for an SMTP email notifier.
type EmailConfig struct {
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// EmailNotifier sends alerts via SMTP email.
type EmailNotifier struct{}

// NewEmailNotifier creates a new email notifier.
func NewEmailNotifier() *EmailNotifier { return &EmailNotifier{} }

func (n *EmailNotifier) Type() string { return "email" }

func (n *EmailNotifier) Send(ctx context.Context, req *NotificationRequest, configBytes []byte) error {
	var cfg EmailConfig
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return fmt.Errorf("parse email config: %w", err)
	}
	if cfg.SMTPHost == "" || cfg.To == "" {
		return fmt.Errorf("email config incomplete")
	}

	subject := fmt.Sprintf("[Lattice Alert] %s - %s", req.RuleName, req.Status)
	body := buildEmailBody(req)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.From, cfg.To, subject, body,
	)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)

	recipients := strings.Split(cfg.To, ",")
	for i, r := range recipients {
		recipients[i] = strings.TrimSpace(r)
	}
	return smtp.SendMail(addr, auth, cfg.From, recipients, []byte(msg))
}

func buildEmailBody(req *NotificationRequest) string {
	lines := []string{
		fmt.Sprintf("Rule: %s", req.RuleName),
		fmt.Sprintf("Status: %s", req.Status),
		fmt.Sprintf("Severity: %s", req.Severity),
		fmt.Sprintf("Time: %s", req.StartedAt.Format(time.RFC3339)),
		"",
	}
	for _, a := range req.Alerts {
		lines = append(lines, fmt.Sprintf("  Value: %.2f", a.Value))
		if a.Message != "" {
			lines = append(lines, fmt.Sprintf("  Message: %s", a.Message))
		}
	}
	return strings.Join(lines, "\n")
}
