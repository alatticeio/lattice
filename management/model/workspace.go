package model

import (
	"time"
	"wireflow/management/dto"

	"gorm.io/gorm"
)

//// 在中间件里判断：
//if GetRoleWeight(member.Role) < GetRoleWeight(model.RoleAdmin) {
//c.AbortWithStatusJSON(403, gin.H{"error": "Insufficient privileges"})
//return
//}

// WorkspaceMember 关联表：连接 User 和 Workspace (Namespace)  这里其实就是RoleBinding 所有的权限校验在数据库层面，不去找k8s的
type WorkspaceMember struct {
	Model
	WorkspaceID string `gorm:"index;column:workspace_id" json:"workspaceID"`
	UserID      string `gorm:"index;column:user_id" json:"userId"`

	// 角色权限
	Role dto.WorkspaceRole `gorm:"type:varchar(20);not null" json:"role"`

	// 成员状态（用于邀请机制）
	Status string `gorm:"type:varchar(20);default:'active'" json:"status"` // e.g., "pending", "active"

	// 关联对象 (GORM 会自动关联，方便预加载)
	User      User      `gorm:"foreignKey:UserID;references:ID" json:"-"`
	Workspace Workspace `gorm:"foreignKey:WorkspaceID;references:ID" json:"-"`
}

func (WorkspaceMember) TableName() string {
	return "t_workspaces_member"
}

// Workspace 结构体：对应 K8s 的 Namespace 用户与User是多对多
type Workspace struct {
	Model

	// 对应用户在前边输入的命名空间, slug并不唯一， 每个用户都可能有test这个空间
	Slug string `gorm:"size:50;not null"` // URL标识，如 "tencent-rd"

	// 物理命名空间：这是关键！对应 K8s metadata.name 用workspace的ID来标识 wf-xxxx-xxxx
	Namespace string `gorm:"type:varchar(63)" json:"namespace"`

	// 显示名称：用户在 Vercel 风格界面看到的名称 (如 "我的私有云")
	DisplayName string `gorm:"type:varchar(100)" json:"displayName"`

	// 状态
	Status  string `gorm:"default:'active'" json:"status"` // active, terminating, frozen
	Members []User `gorm:"many2many:t_workspace_member;" json:"members,omitempty"`
}

func (Workspace) TableName() string {
	return "t_workspace"
}

func (w *Workspace) SetNamespace(ns string) {
	// 只有在 Namespace 为空时才自动同步，避免覆盖前端手动传入的值
	if w.Namespace == "" {
		w.Namespace = ns
	}
}

// Plan 定义不同等级套餐的“物理限制”和“功能开关”
type Plan struct {
	ID                uint   `gorm:"primaryKey"`
	Name              string `gorm:"unique;not null"` // e.g., "free", "pro", "enterprise"
	MaxWorkspaces     int    `gorm:"default:1"`       // 允许创建的工作空间数
	MaxNodesPerWS     int    `gorm:"default:5"`       // 每个空间允许接入的边缘节点数
	AllowCustomDNS    bool   `gorm:"default:false"`   // 是否允许配置自定义 DNS
	AllowMeshTopology bool   `gorm:"default:false"`   // 是否允许全互联拓扑
	PeerLimit         string `gorm:"default:0"`
	CpuLimit          string `gorm:"default:500m"`  // 对应 K8s ResourceQuota
	MemoryLimit       string `gorm:"default:512Mi"` // 对应 K8s ResourceQuota
}

// Subscription 记录用户当前执行的套餐状态
type Subscription struct {
	gorm.Model
	UserID    uint `gorm:"uniqueIndex"` // 每个用户同一时间只有一个订阅记录
	PlanID    uint
	Plan      Plan      `gorm:"foreignKey:PlanID"`
	Status    string    // "active", "expired", "past_due"
	ExpiresAt time.Time // 过期时间
}
