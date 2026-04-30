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

	// 所属工作空间（空 = 平台级）
	WorkspaceID string `gorm:"index;size:36" json:"workspaceId"`

	// 申请人（写入时冗余，避免联表）
	RequestedBy      string `gorm:"index;size:36" json:"requestedBy"`
	RequestedByName  string `gorm:"size:100"      json:"requestedByName"`
	RequestedByEmail string `gorm:"size:254"      json:"requestedByEmail"`

	// 操作描述
	ResourceType string `gorm:"size:50;index" json:"resourceType"` // policy | member | relay | ...
	ResourceName string `gorm:"size:200"      json:"resourceName"`
	Action       string `gorm:"size:50;index" json:"action"` // create | update | delete

	// 操作载体（JSON 快照），executor 读取后执行真实 K8s/DB 操作
	Payload string `gorm:"type:text" json:"payload"`

	// 状态机
	Status WorkflowStatus `gorm:"size:20;index;default:'pending'" json:"status"`

	// 审批信息
	ReviewedBy     string     `gorm:"size:36"  json:"reviewedBy,omitempty"`
	ReviewedByName string     `gorm:"size:100" json:"reviewedByName,omitempty"`
	ReviewedAt     *time.Time `json:"reviewedAt,omitempty"`
	ReviewNote     string     `gorm:"size:500" json:"reviewNote,omitempty"`

	// 执行信息
	ExecutedAt   *time.Time `json:"executedAt,omitempty"`
	ErrorMessage string     `gorm:"size:1000" json:"errorMessage,omitempty"`
}

func (WorkflowRequest) TableName() string { return "t_workflow_request" }
