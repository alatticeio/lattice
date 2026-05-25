package models

import (
	"github.com/alatticeio/lattice/internal/server/dto"
	"time"

	"gorm.io/gorm"
)

//// Check in middleware:
//if GetRoleWeight(member.Role) < GetRoleWeight(model.RoleAdmin) {
//c.AbortWithStatusJSON(403, gin.H{"error": "Insufficient privileges"})
//return
//}

// WorkspaceMember status constants.
const (
	MemberStatusActive    = "active"
	MemberStatusPending   = "pending"
	MemberStatusSuspended = "suspended"
	MemberStatusRemoved   = "removed"
)

// WorkspaceMember join table: connects User and Workspace (Namespace) - this is essentially RoleBinding; all permission checks happen at the DB level, not via K8s.
type WorkspaceMember struct {
	Model
	WorkspaceID string `gorm:"index;column:workspace_id" json:"workspaceID"`
	UserID      string `gorm:"index;column:user_id" json:"userId"`

	// Role permissions
	Role dto.WorkspaceRole `gorm:"type:varchar(20);not null" json:"role"`

	// Member status (for invitation mechanism)
	Status string `gorm:"type:varchar(20);default:'active'" json:"status"` // e.g., "pending", "active"

	InvitedBy string     `gorm:"column:invited_by" json:"invitedBy,omitempty"`
	JoinedAt  *time.Time `gorm:"column:joined_at" json:"joinedAt,omitempty"`

	// Related objects (GORM auto-associates, convenient for preloading)
	User      User      `gorm:"foreignKey:UserID;references:ID" json:"-"`
	Workspace Workspace `gorm:"foreignKey:WorkspaceID;references:ID" json:"-"`
}

func (WorkspaceMember) TableName() string {
	return "t_workspaces_member"
}

// Workspace struct: corresponds to K8s Namespace; many-to-many relationship with User
type Workspace struct {
	Model

	// Corresponds to the namespace the user entered; slug is not unique - each user may have a "test" workspace
	Slug string `gorm:"size:50;not null"` // URL identifier, e.g. "tencent-rd"

	// Physical namespace: this is the key! Corresponds to K8s metadata.name, using the workspace ID as wf-xxxx-xxxx
	Namespace string `gorm:"type:varchar(63)" json:"namespace"`

	// Display name: the name users see in the Vercel-style UI (e.g. "My Private Cloud")
	DisplayName string `gorm:"type:varchar(100)" json:"displayName"`

	// Node quota
	MaxNodeCount int `gorm:"default:10" json:"maxNodeCount"`

	// Status
	Status  string `gorm:"default:'active'" json:"status"` // active, terminating, frozen
	Members []User `gorm:"-" json:"members,omitempty"`

	// Operator
	CreatedBy string `gorm:"type:varchar(100)" json:"createdBy,omitempty"`
	UpdatedBy string `gorm:"type:varchar(100)" json:"updatedBy,omitempty"`

	// Playground seed data
	SeedInjected bool `gorm:"default:false" json:"seedInjected"`
}

func (Workspace) TableName() string {
	return "t_workspace"
}

func (w *Workspace) SetNamespace(ns string) {
	// Only auto-sync when Namespace is empty, to avoid overwriting frontend-provided values
	if w.Namespace == "" {
		w.Namespace = ns
	}
}

// WorkspaceInvitation tracks a pending invitation to join a workspace.
type WorkspaceInvitation struct {
	Model
	WorkspaceID string            `gorm:"index;not null" json:"workspaceId"`
	InviterID   string            `json:"inviterId"`
	Email       string            `json:"email"` // invitee email (may not be a user yet)
	Role        dto.WorkspaceRole `gorm:"type:varchar(20)" json:"role"`
	Token       string            `gorm:"uniqueIndex" json:"token"`                         // secure random hex token
	Status      string            `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending|accepted|expired|revoked
	ExpiresAt   time.Time         `json:"expiresAt"`
}

func (WorkspaceInvitation) TableName() string { return "t_workspace_invitation" }

// Plan defines the “resource limits” and “feature toggles” for different plan tiers
type Plan struct {
	ID                uint   `gorm:"primaryKey"`
	Name              string `gorm:"unique;not null"` // e.g., "free", "pro", "enterprise"
	MaxWorkspaces     int    `gorm:"default:1"`       // Maximum number of workspaces allowed
	MaxNodesPerWS     int    `gorm:"default:5"`       // Maximum edge nodes per workspace
	AllowCustomDNS    bool   `gorm:"default:false"`   // Whether custom DNS is allowed
	AllowMeshTopology bool   `gorm:"default:false"`   // Whether full-mesh topology is allowed
	PeerLimit         string `gorm:"default:0"`
	CpuLimit          string `gorm:"default:500m"`  // Corresponds to K8s ResourceQuota
	MemoryLimit       string `gorm:"default:512Mi"` // Corresponds to K8s ResourceQuota
}

// Subscription records the user's current subscription plan status
type Subscription struct {
	gorm.Model
	UserID    uint `gorm:"uniqueIndex"` // Each user has only one active subscription at a time
	PlanID    uint
	Plan      Plan      `gorm:"foreignKey:PlanID"`
	Status    string    // "active", "expired", "past_due"
	ExpiresAt time.Time // Expiration time
}
