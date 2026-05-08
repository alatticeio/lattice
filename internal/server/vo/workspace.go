package vo

import "github.com/alatticeio/lattice/internal/server/dto"

// MemberVo is the HTTP response type for a workspace member, combining WorkspaceMember + User fields.
type MemberVo struct {
	UserID   string            `json:"userId"`
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Avatar   string            `json:"avatar"`
	Role     dto.WorkspaceRole `json:"role"`
	Provider string            `json:"provider"` // "local", "dex", "ldap", etc.
	Status   string            `json:"status"`   // "active", "pending"
	JoinedAt string            `json:"joinedAt,omitempty"`
}

type WorkspaceVo struct {
	ID string `json:"id"`

	// slug corresponds to the workspace name entered in the frontend
	Slug string `json:"slug"` // URL identifier, e.g. "tencent-rd"

	// Corresponds to the K8s namespace
	Namespace string `json:"namespace"`

	// Display name: the name users see in the Vercel-style UI (e.g. "My Private Cloud")
	DisplayName string `json:"displayName"`

	TokenCount int64 `json:"tokenCount"`

	QuotaUsage int64 `json:"quotaUsage"`

	NodeCount int64 `json:"nodeCount"`

	// Status
	Status string `json:"status"` // active, terminating, frozen

	// Creation time
	CreatedAt string `json:"createdAt,omitempty"`

	// Creator
	CreatedBy string `json:"createdBy,omitempty"`

	// Last modifier and modification time
	UpdatedBy string `json:"updatedBy,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`

	// Network info
	NetworkName   string `json:"networkName,omitempty"`
	NetworkCIDR   string `json:"networkCIDR,omitempty"`
	NetworkStatus string `json:"networkStatus,omitempty"`

	Members []UserVo `json:"members,omitempty"`
}
