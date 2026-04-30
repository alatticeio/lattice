package models

type PolicyStatus string

const (
	PolicyStatusPending  PolicyStatus = "pending"  // awaiting approval
	PolicyStatusApproved PolicyStatus = "approved" // approved, executor running
	PolicyStatusActive   PolicyStatus = "active"   // applied to k8s
	PolicyStatusFailed   PolicyStatus = "failed"   // executor failed
)

// Policy is the database record for a LatticePolicy.
// It serves as the source of truth for the policy list, capturing both
// pending (pre-approval) and active (deployed to k8s) policies.
type Policy struct {
	Model
	WorkspaceID       string       `gorm:"size:36;uniqueIndex:idx_policy_ws_name"     json:"workspaceId"`
	Name              string       `gorm:"size:200;uniqueIndex:idx_policy_ws_name"    json:"name"`
	Description       string       `gorm:"size:500"                                   json:"description"`
	Action            string       `gorm:"size:20"                                    json:"action"`
	PolicyTypes       string       `gorm:"type:text"                                  json:"policyTypes"` // JSON: ["Ingress","Egress"]
	Spec              string       `gorm:"type:text"                                  json:"spec"`        // JSON of LatticePolicySpec
	Status            PolicyStatus `gorm:"size:30;default:'pending';index"            json:"status"`
	WorkflowRequestID string       `gorm:"size:36"                                    json:"workflowRequestId,omitempty"`
	ErrorMessage      string       `gorm:"size:500"                                   json:"errorMessage,omitempty"`
	CreatedBy         string       `gorm:"size:36"                                    json:"createdBy,omitempty"`
	CreatedByName     string       `gorm:"size:200"                                   json:"createdByName,omitempty"`
	UpdatedBy         string       `gorm:"size:36"                                    json:"updatedBy,omitempty"`
	UpdatedByName     string       `gorm:"size:200"                                   json:"updatedByName,omitempty"`
}

func (Policy) TableName() string { return "t_policy" }
