package models

import "time"

// AgentIdentity stores AI Agent identity records for non-K8s deployments.
// Each agent gets an independent identity with tool RBAC and sandbox settings.
type AgentIdentity struct {
	Model

	TenantID          string     `gorm:"size:36;index;not null" json:"tenant_id"`
	Name              string     `gorm:"size:200;not null" json:"name"`
	PeerRef           string     `gorm:"size:200;not null" json:"peer_ref"`
	AllowedTools      string     `gorm:"type:text" json:"allowed_tools,omitempty"`      // JSON array
	AllowedNamespaces string     `gorm:"type:text" json:"allowed_namespaces,omitempty"` // JSON array
	Sandbox           string     `gorm:"size:50;not null;default:none" json:"sandbox"`
	AuditLevel        string     `gorm:"size:50;not null;default:write" json:"audit_level"`
	EnforcementMode   string     `gorm:"size:50;not null;default:enforce" json:"enforcement_mode"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	ParentRef         string     `gorm:"size:200" json:"parent_ref,omitempty"`
	SpawnableRoles    string     `gorm:"type:text" json:"spawnable_roles,omitempty"` // JSON array
	Phase             string     `gorm:"size:50;not null;default:Pending" json:"phase"`
	PeerIP            string     `gorm:"size:64" json:"peer_ip,omitempty"`
	LastSeenAt        *time.Time `json:"last_seen_at,omitempty"`
	Description       string     `gorm:"size:500" json:"description,omitempty"`
}

func (AgentIdentity) TableName() string { return "t_agent_identity" }
