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

	// slug对应前端输入的想要的空间
	Slug string `json:"slug"` // URL标识，如 "tencent-rd"

	//对应k8s的 namespace
	Namespace string `json:"namespace"`

	// 显示名称：用户在 Vercel 风格界面看到的名称 (如 "我的私有云")
	DisplayName string `json:"displayName"`

	TokenCount int64 `json:"tokenCount"`

	QuotaUsage int64 `json:"quotaUsage"`

	NodeCount int64 `json:"nodeCount"`

	// 状态
	Status string `json:"status"` // active, terminating, frozen

	// 创建时间
	CreatedAt string `json:"createdAt,omitempty"`

	// 创建人
	CreatedBy string `json:"createdBy,omitempty"`

	// 最后修改人和修改时间
	UpdatedBy string `json:"updatedBy,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`

	// 网络信息
	NetworkName   string `json:"networkName,omitempty"`
	NetworkCIDR   string `json:"networkCIDR,omitempty"`
	NetworkStatus string `json:"networkStatus,omitempty"`

	Members []UserVo `json:"members,omitempty"`
}
