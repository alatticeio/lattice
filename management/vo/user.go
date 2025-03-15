package vo

import (
	"linkany/management/utils"
	"time"
)

type InviteVo struct {
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
	ID            uint64         `json:"id" :"id"`
	Group         string         `json:"group,omitempty" :"group"`
	InviterName   string         `json:"inviterName,omitempty" :"inviter_name"`
	InviterAvatar string         `json:"inviterAvatar,omitempty"`
	Role          string         `json:"role,omitempty"`
	Permissions   string         `json:"permissions,omitempty"`
	AcceptStatus  string         `json:"acceptStatus,omitempty"`
	InvitedAt     utils.NullTime `json:"invitedAt,omitempty"`
}
