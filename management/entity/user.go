package entity

import (
	"linkany/management/utils"
)

type User struct {
	Model
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
	Model
	InvitationId    uint64             // 分配的用户
	OwnerId         uint64             // 资源所有者,也即是邀请者
	InviteId        uint64             //关联的邀请表主键
	ResourceType    utils.ResourceType //资源类型
	ResourceId      uint64             //资源id
	PermissionText  string             //添加组
	PermissionValue string             //group:add
	PermissionId    uint64             //group:add

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
