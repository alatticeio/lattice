package vo

import (
	"linkany/management/utils"
	"time"
)

type UserVo struct {
	ID          uint64 `json:"id,string"`
	Username    string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	MobilePhone string `json:"mobilePhone,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	Address     string `json:"address,omitempty"`
}

type InviteVo struct {
	*UserResourceVo
	ID           uint64         `json:"id"`
	InviteeName  string         `json:"inviteeName,omitempty"`
	InviterName  string         `json:"inviterName,omitempty"`
	MobilePhone  string         `json:"mobilePhone,omitempty"`
	Email        string         `json:"email,omitempty"`
	Role         string         `json:"role,omitempty"`
	Avatar       string         `json:"avatar,omitempty"`
	GroupId      uint64         `json:"groupId,omitempty"`
	GroupName    string         `json:"groupName,omitempty"`
	Permissions  string         `json:"permissions,omitempty"`
	AcceptStatus string         `json:"acceptStatus,omitempty"`
	InvitedAt    time.Time      `json:"invitedAt,omitempty"`
	CanceledAt   utils.NullTime `json:"canceledAt,omitempty"`
}

type InvitationVo struct {
	ID            uint64         `json:"id,string"`
	Group         string         `json:"group,omitempty"`
	InviterName   string         `json:"inviterName,omitempty"`
	InviterAvatar string         `json:"inviterAvatar,omitempty"`
	InviteId      uint64         `json:"inviteId,string"`
	Role          string         `json:"role,omitempty"`
	Permissions   string         `json:"permissions,omitempty"`
	AcceptStatus  string         `json:"acceptStatus,omitempty"`
	InvitedAt     utils.NullTime `json:"invitedAt,omitempty"`
}

type NodeResourceVo struct {
	NodeValues map[string]string `json:"nodeValues"`
}

type PolicyResourceVo struct {
	PolicyValues map[string]string `json:"policyValues" gorm:"-"`
}

type GroupResourceVo struct {
	GroupValues map[string]string `json:"groupValues"`
}

type PermissionResourceVo struct {
	PermissionValues map[string]string `json:"permissionValues"`
}

type LabelResourceVo struct {
	LabelValues map[string]string `json:"labelValues"`
}

type UserResourceVo struct {
	*GroupResourceVo
	*PolicyResourceVo
	*NodeResourceVo
	*PermissionResourceVo
	*LabelResourceVo
}
