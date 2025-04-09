package entity

import (
	"linkany/management/utils"

	"gorm.io/gorm"
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

// UserResourceGrantedPermission a user's permission which granted by owner. focus on the resources created by owner.
// resource level
type UserResourceGrantedPermission struct {
	gorm.Model
	InvitationId    uint               // 分配的用户
	OwnerId         uint               // 资源所有者,也即是邀请者
	InviteId        uint               //关联的邀请表主键
	ResourceType    utils.ResourceType //资源类型
	ResourceId      uint               //资源id
	PermissionText  string             //添加组
	PermissionValue string             //group:add
	PermissionId    uint               //group:add

	AcceptStatus AcceptStatus
}

func (UserResourceGrantedPermission) TableName() string {
	return "la_user_resource_granted_permission"
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
