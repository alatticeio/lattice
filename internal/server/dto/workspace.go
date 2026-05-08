package dto

type WorkspaceDto struct {
	Slug string `json:"slug"` // URL identifier, e.g. "tencent-rd"

	// Physical namespace: this is key! Corresponds to K8s metadata.name
	// Must comply with DNS-1123 format (lowercase letters, numbers, hyphens)
	Namespace string `json:"namespace"`

	// Display name: the name users see in the Vercel-style UI (e.g. "My Private Cloud")
	DisplayName string `json:"displayName"`

	// Workspace quota
	MaxNodeCount int `json:"maxNodeCount"`
}

// WorkspaceRole defines team role types
type WorkspaceRole string

const (
	RoleAdmin  WorkspaceRole = "admin"  // K8s equivalent: admin, can manage members and resources
	RoleEditor WorkspaceRole = "editor" // K8s equivalent: editor, can operate resources but not manage members
	RoleMember WorkspaceRole = "member"
	RoleViewer WorkspaceRole = "viewer" // K8s equivalent: viewer, read-only

)

func GetRoleWeight(role WorkspaceRole) int {
	weights := map[WorkspaceRole]int{
		RoleAdmin:  100,
		RoleEditor: 80,
		RoleMember: 40,
		RoleViewer: 10,
	}
	return weights[role]
}

// SystemRole defines platform-level roles (not workspace-level).
type SystemRole string

const (
	SystemRolePlatformAdmin SystemRole = "platform_admin"
	SystemRoleUser          SystemRole = "user"
)
