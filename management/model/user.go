package model

import "wireflow/management/dto"

// User 结构体：对应用户名密码以及外部 SSO 同步进来的用户
type User struct {
	Model
	Role       dto.WorkspaceRole `gorm:"type:varchar(20)" json:"role"`
	ExternalID string            `gorm:"column:external_id" json:"external_id"`
	Username   string            `json:"username,omitempty"`
	Password   string            `json:"password,omitempty"`
	Mobile     string            `json:"mobile,omitempty"`
	Email      string            `json:"email"`
	Avatar     string            `json:"avatar"`
	Address    string            `json:"address,omitempty"`
	Gender     int               `json:"gender,omitempty"`
	Workspaces []Workspace       `gorm:"many2many:t_workspaces_member;" json:"workspaces,omitempty"`
}

func (User) TableName() string {
	return "t_user"
}

type Namespace struct {
	Model
	Name        string `gorm:"name"`
	DisplayName string `gorm:"display_name"`
}

func (n Namespace) TableName() string {
	return "t_namespace"
}

type UserNamespacePermission struct {
	Model
	UserID      string `gorm:"user_id" json:"user_id"`
	Namespace   string `gorm:"namespace" json:"namespace"`
	AccessLevel string `gorm:"access_level" json:"level"` // "read", "write", "admin"
}

func (UserNamespacePermission) TableName() string {
	return "t_user_namespace_permission"
}
