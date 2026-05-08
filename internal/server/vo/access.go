package vo

import (
	"time"
)

type AccessPolicyVo struct {
	ID          uint64    `json:"id"`
	Name        string    `json:"name"`                  // Policy name
	GroupID     uint64    `json:"group_id"`              // Group ID
	Priority    int       `json:"priority"`              // Policy priority (higher number = higher priority)
	Effect      string    `json:"effect"`                // Effect: allow/deny
	Description string    `json:"description,omitempty"` // Policy description
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	CreatedBy   string    `json:"createdBy"` // Creator
	UpdatedBy   string    `json:"updatedBy"`
	DeletedAt   time.Time `json:"deletedAt"`

	Rules []*AccessRuleVo `json:"rules"`
}

type AccessRuleVo struct {
	ID         uint64    `json:"id"`
	Name       string    `json:"name"`                 // Rule name
	PolicyID   uint64    `json:"policyId"`             // Parent policy ID
	SourceType string    `json:"sourceType"`           // Source type: node/tag/all
	SourceID   string    `json:"sourceId"`             // Source identifier (node ID or tag)
	TargetType string    `json:"targetType"`           // Target type: node/tag/all
	TargetID   string    `json:"targetId"`             // Target identifier (node ID or tag)
	Actions    string    `json:"actions"`              // Allowed actions list
	TimeType   string    `json:"timeType"`             // Time type
	Conditions string    `json:"conditions,omitempty"` // Additional conditions (e.g. time limits, bandwidth limits)
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	CreatedBy  string    `json:"createdBy"`
	UpdatedBy  string    `json:"updatedBy"`
	DeletedAt  time.Time `json:"deletedAt"`
}
