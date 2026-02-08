package model

// RoleAssignment 用户对应权限
type RoleAssignment struct {
	Model
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID uint   `gorm:"index;not null" json:"user_id"` // 关联用户
	Role   string `gorm:"size:50;not null" json:"role"`  // 角色标识：admin, editor, viewer

	// 作用域设计
	ScopeType string `gorm:"size:20;not null" json:"scope_type"` // GLOBAL 或 NAMESPACE
	ScopeID   string `gorm:"size:100;index" json:"scope_id"`     // 如果是 GLOBAL，填 "system"；如果是 NAMESPACE，填空间ID

	// 关联预加载（可选，方便查询时直接拿到用户信息）
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

const (
	// 作用域类型
	ScopeGlobal    = "GLOBAL"
	ScopeNamespace = "NAMESPACE"

	// 系统层角色
	RolePlatformAdmin = "platform_admin" // 系统管理员
	RolePlatformUser  = "platform_user"  // 普通系统成员

	// 空间层角色
	RoleNSAdmin  = "ns_admin"  // 空间管理员
	RoleNSEditor = "ns_editor" // 空间编辑
	RoleNSViewer = "ns_viewer" // 空间只读
)
