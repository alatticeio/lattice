package models

import "time"

// AuditLog records every mutating operation in the system.
// It is append-only: no UpdatedAt, no soft-delete.
type AuditLog struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CreatedAt time.Time `gorm:"index"                       json:"createdAt"`

	// 操作者
	UserID    string `gorm:"index;size:36"  json:"userId"`
	UserName  string `gorm:"size:100"       json:"userName"`  // denormalized for display
	UserEmail string `gorm:"size:254"       json:"userEmail"` // fallback display when userName is empty
	UserIP    string `gorm:"size:45"        json:"userIP"`

	// 作用域
	WorkspaceID string `gorm:"index;size:36" json:"workspaceId"` // empty = platform-level

	// 操作描述
	Action       string `gorm:"size:50;index" json:"action"`       // CREATE UPDATE DELETE LOGIN INVITE REVOKE EXPORT
	Resource     string `gorm:"size:50;index" json:"resource"`     // member workspace policy token relay invitation peer
	ResourceID   string `gorm:"size:36"       json:"resourceId"`
	ResourceName string `gorm:"size:200"      json:"resourceName"` // denormalized

	// 影响范围 — 描述本次操作波及的对象或数量，e.g. "成员: alice@example.com → 角色: editor"
	Scope string `gorm:"size:500" json:"scope"`

	// 结果
	Status     string `gorm:"size:20;default:'success'" json:"status"` // success | failed
	StatusCode int    `json:"statusCode"`

	// 详情快照（JSON，可选，仅记录关键操作的 before/after）
	Detail string `gorm:"type:text" json:"detail,omitempty"`
}

func (AuditLog) TableName() string { return "t_audit_log" }
