package vo

import (
	"linkany/management/utils"
	"time"
)

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
	InviteId      uint           `json:"inviteId,string"`
	Role          string         `json:"role,omitempty"`
	Permissions   string         `json:"permissions,omitempty"`
	AcceptStatus  string         `json:"acceptStatus,omitempty"`
	InvitedAt     utils.NullTime `json:"invitedAt,omitempty"`
}

type NodeResourceVo struct {
	NodeIds   []uint   `json:"nodeIds"`
	NodeNames []string `json:"nodeNames"`
}

type PolicyResourceVo struct {
	PolicyIds   []uint   `json:"policyIds"`
	PolicyNames []string `json:"policyNames"`
}

type GroupResourceVo struct {
	GroupIds   []uint   `json:"groupIds"`
	GroupNames []string `json:"groupNames"`
}

type PermissionResourceVo struct {
	PermissionIds   []uint   `json:"permissionIds"`
	PermissionNames []string `json:"permissionNames"`
}

type LabelResourceVo struct {
	LabelIds   []uint   `json:"labelIds"`
	LabelNames []string `json:"labelNames"`
}

type UserResourceVo struct {
	*GroupResourceVo
	*PolicyResourceVo
	*NodeResourceVo
	*PermissionResourceVo
	*LabelResourceVo
}
