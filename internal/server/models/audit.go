package models

import "time"

// AuditLog records every mutating operation in the system.
// It is append-only: no UpdatedAt, no soft-delete.
type AuditLog struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CreatedAt time.Time `gorm:"index"                       json:"createdAt"`

	// Operator
	UserID    string `gorm:"index;size:36"  json:"userId"`
	UserName  string `gorm:"size:100"       json:"userName"`  // denormalized for display
	UserEmail string `gorm:"size:254"       json:"userEmail"` // fallback display when userName is empty
	UserIP    string `gorm:"size:45"        json:"userIP"`

	// Scope
	WorkspaceID string `gorm:"index;size:36" json:"workspaceId"` // empty = platform-level

	// Operation description
	Action       string `gorm:"size:50;index" json:"action"`   // CREATE UPDATE DELETE LOGIN INVITE REVOKE EXPORT
	Resource     string `gorm:"size:50;index" json:"resource"` // member workspace policy token relay invitation peer
	ResourceID   string `gorm:"size:36"       json:"resourceId"`
	ResourceName string `gorm:"size:200"      json:"resourceName"` // denormalized

	// Scope — describes the objects or counts affected by this operation, e.g. "member: alice@example.com → role: editor"
	Scope string `gorm:"size:500" json:"scope"`

	// Result
	Status     string `gorm:"size:20;default:'success'" json:"status"` // success | failed
	StatusCode int    `json:"statusCode"`

	// Detail snapshot (JSON, optional, only records before/after for critical operations)
	Detail string `gorm:"type:text" json:"detail,omitempty"`
}

func (AuditLog) TableName() string { return "t_audit_log" }
