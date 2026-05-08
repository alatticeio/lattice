package models

import "time"

// WorkflowStatus represents the lifecycle of an approval request.
type WorkflowStatus string

const (
	WorkflowStatusPending  WorkflowStatus = "pending"
	WorkflowStatusApproved WorkflowStatus = "approved"
	WorkflowStatusRejected WorkflowStatus = "rejected"
	WorkflowStatusExecuted WorkflowStatus = "executed"
	WorkflowStatusFailed   WorkflowStatus = "failed"
)

// WorkflowRequest records a user action that requires approval before execution.
// Once approved, a background executor picks it up and performs the real operation.
type WorkflowRequest struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CreatedAt time.Time `gorm:"index"                       json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Parent workspace (empty = platform-level)
	WorkspaceID string `gorm:"index;size:36" json:"workspaceId"`

	// Requester (redundant at write time to avoid joins)
	RequestedBy      string `gorm:"index;size:36" json:"requestedBy"`
	RequestedByName  string `gorm:"size:100"      json:"requestedByName"`
	RequestedByEmail string `gorm:"size:254"      json:"requestedByEmail"`

	// Operation description
	ResourceType string `gorm:"size:50;index" json:"resourceType"` // policy | member | relay | ...
	ResourceName string `gorm:"size:200"      json:"resourceName"`
	Action       string `gorm:"size:50;index" json:"action"` // create | update | delete

	// Operation payload (JSON snapshot); executor reads it and performs real K8s/DB operations
	Payload string `gorm:"type:text" json:"payload"`

	// State machine
	Status WorkflowStatus `gorm:"size:20;index;default:'pending'" json:"status"`

	// Approval info
	ReviewedBy     string     `gorm:"size:36"  json:"reviewedBy,omitempty"`
	ReviewedByName string     `gorm:"size:100" json:"reviewedByName,omitempty"`
	ReviewedAt     *time.Time `json:"reviewedAt,omitempty"`
	ReviewNote     string     `gorm:"size:500" json:"reviewNote,omitempty"`

	// Execution info
	ExecutedAt   *time.Time `json:"executedAt,omitempty"`
	ErrorMessage string     `gorm:"size:1000" json:"errorMessage,omitempty"`
}

func (WorkflowRequest) TableName() string { return "t_workflow_request" }
