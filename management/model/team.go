package model

// TeamRole 定义团队角色类型
type TeamRole string

const (
	RoleOwner  TeamRole = "admin"  // 对应 K8s: 管理员，可管理成员和资源
	RoleEditor TeamRole = "editor" // 对应 K8s: 编辑者，可操作资源但不能管理成员
	RoleViewer TeamRole = "viewer" // 对应 K8s: 观察者，仅只读权限
)

// TeamMember 关联表：连接 User 和 Team (Namespace)
type TeamMember struct {
	Model
	TeamID string `gorm:"primaryKey;index;column:team_id" json:"teamId"`
	UserID string `gorm:"primaryKey;index;column:user_id" json:"userId"`

	// 角色权限
	Role TeamRole `gorm:"type:varchar(20);not null" json:"role"`

	// 成员状态（用于邀请机制）
	Status string `gorm:"type:varchar(20);default:'active'" json:"status"` // e.g., "pending", "active"

	// 关联对象 (GORM 会自动关联，方便预加载)
	User User `gorm:"foreignKey:UserID" json:"-"`
	Team Team `gorm:"foreignKey:TeamID" json:"-"`
}

// Team 结构体：对应 K8s 的 Namespace
type Team struct {
	Model
	// 物理命名空间：这是关键！对应 K8s metadata.name
	// 必须符合 DNS-1123 规范（小写字母、数字、中划线）
	Namespace string `gorm:"uniqueIndex;type:varchar(63)" json:"namespace"`

	// 显示名称：用户在 Vercel 风格界面看到的名称 (如 "我的私有云")
	DisplayName string `gorm:"type:varchar(100)" json:"displayName"`

	// 状态
	Status  string `gorm:"default:'active'" json:"status"` // active, terminating, frozen
	Members []User `gorm:"many2many:team_members;" json:"members,omitempty"`
}
