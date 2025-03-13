package entity

import (
	"gorm.io/gorm"
	"linkany/management/utils"
)

type User struct {
	gorm.Model
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Mobile   string `json:"mobile,omitempty"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	Address  string `json:"address,omitempty"`
	Gender   int    `json:"gender,omitempty"`
}

// UserGroupShared give a user groups permit
type UserGroupShared struct {
	gorm.Model
	OwnerId     uint
	UserId      uint
	GroupId     uint
	Description string
}

// UserResourceGrantedPermission a user's permission which granted by owner. focus on the resources created by owner.
// resource level
type UserResourceGrantedPermission struct {
	gorm.Model
	InvitedId     uint               // 分配的用户
	OwnerId       uint               // 资源所有者
	ResourceType  utils.ResourceType //资源类型
	ResourceId    string             //资源id
	Permission    string             //group:add
	PermissionIds string             //group:add
}

func (UserResourceGrantedPermission) TableName() string {
	return "la_user_resource_granted_permission"
}

// UserGrantedPermission user granted permission
// whole level
type UserGrantedPermission struct {
	gorm.Model
	OwnId      uint
	InvitedId  uint
	Permission string
}

// granted role's permissions will add here
func (UserGrantedPermission) TableName() string {
	return "la_user_granted_permission"
}

func (u *User) TableName() string {
	return "la_user"
}

type Token struct {
	Token  string `json:"token,omitempty"`
	Avatar string `json:"avatar,omitempty"`
	Email  string `json:"email,omitempty"`
	Mobile string `json:"mobile,omitempty"`
}
