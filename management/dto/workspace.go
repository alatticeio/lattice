package dto

type WorkspaceDto struct {
	Slug string `json:"slug"` // URL标识，如 "tencent-rd"

	// 物理命名空间：这是关键！对应 K8s metadata.name
	// 必须符合 DNS-1123 规范（小写字母、数字、中划线）
	Namespace string `json:"namespace"`

	// 显示名称：用户在 Vercel 风格界面看到的名称 (如 "我的私有云")
	DisplayName string `json:"displayName"`

	// 空间配额
	MaxNodeCount int `json:"maxNodeCount"`
}

// WorkspaceRole 定义团队角色类型
type WorkspaceRole string

const (
	RoleAdmin  WorkspaceRole = "admin"  // 对应 K8s: 管理员，可管理成员和资源
	RoleEditor WorkspaceRole = "editor" // 对应 K8s: 编辑者，可操作资源但不能管理成员
	RoleMember WorkspaceRole = "member"
	RoleViewer WorkspaceRole = "viewer" // 对应 K8s: 观察者，仅只读权限

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
