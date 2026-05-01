package models

import "time"

// AlertRule defines a monitoring alert rule.
type AlertRule struct {
	Model

	Name        string `gorm:"not null" json:"name"`
	WorkspaceID string `gorm:"index;not null" json:"workspace_id"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`

	MetricType string  `gorm:"not null" json:"metric_type"`
	Operator   string  `gorm:"not null" json:"operator"`
	Threshold  float64 `gorm:"not null" json:"threshold"`
	Duration   string  `gorm:"not null" json:"duration"`
	Lookback   string  `gorm:"not null" json:"lookback"`

	GroupBy string `gorm:"type:text" json:"group_by"`
	ForEach bool   `gorm:"default:false" json:"for_each"`

	Channels string `gorm:"type:text" json:"channels"`
	Severity string `gorm:"not null" json:"severity"`
	Message  string `gorm:"type:text" json:"message"`

	SilenceUntil  *time.Time `json:"silence_until"`
	MuteTimeRules string     `gorm:"type:text" json:"mute_time_rules"`
}

// AlertHistory records the firing and resolution of alerts.
type AlertHistory struct {
	Model

	RuleID      string     `gorm:"index;not null" json:"rule_id"`
	WorkspaceID string     `gorm:"index;not null" json:"workspace_id"`
	Status      string     `gorm:"not null" json:"status"`
	Severity    string     `gorm:"not null" json:"severity"`
	Labels      string     `gorm:"type:text" json:"labels"`
	Value       float64    `json:"value"`
	Message     string     `gorm:"type:text" json:"message"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
	Notified    bool       `gorm:"default:false" json:"notified"`
}

// AlertChannel defines a notification destination.
type AlertChannel struct {
	Model

	Name        string `gorm:"not null" json:"name"`
	WorkspaceID string `gorm:"index;not null" json:"workspace_id"`
	Type        string `gorm:"not null" json:"type"`
	Config      string `gorm:"type:text" json:"config"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`
}

// AlertSilence temporarily mutes alerts matching certain criteria.
type AlertSilence struct {
	Model

	WorkspaceID string    `gorm:"index;not null" json:"workspace_id"`
	CreatedBy   string    `json:"created_by"`
	Matchers    string    `gorm:"type:text" json:"matchers"`
	Comment     string    `json:"comment"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
}
