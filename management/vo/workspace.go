package vo

type WorkspaceVo struct {
	ID string `json:"id"`

	// slug对应前端输入的想要的空间
	Slug string `json:"name"'` // URL标识，如 "tencent-rd"

	//对应k8s的 namespace
	Namespace string `json:"namespace"`

	// 显示名称：用户在 Vercel 风格界面看到的名称 (如 "我的私有云")
	DisplayName string `json:"displayName"`

	TokenCount int64 `json:"tokenCount"`

	QuotaUsage int64 `json:"quotaUsage"`

	NodeCount int64 `json:"nodeCount"`

	// 状态
	Status string `json:"status"` // active, terminating, frozen

	Members []UserVo `json:"members,omitempty"`
}
