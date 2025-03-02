package vo

import (
	"linkany/management/entity"
	"time"
)

type InviteVo struct {
	ID           uint64          `json:"id"`
	InviteeName  string          `json:"inviteeName,omitempty"`
	InviterName  string          `json:"inviterName,omitempty"`
	MobilePhone  string          `json:"mobilePhone,omitempty"`
	Email        string          `json:"email,omitempty"`
	Role         string          `json:"role,omitempty"`
	Avatar       string          `json:"avatar,omitempty"`
	GroupId      uint64          `json:"groupId,omitempty"`
	GroupName    string          `json:"groupName,omitempty"`
	Permissions  string          `json:"permissions,omitempty"`
	AcceptStatus string          `json:"acceptStatus,omitempty"`
	InvitedAt    time.Time       `json:"invitedAt,omitempty"`
	CanceledAt   entity.NullTime `json:"canceledAt,omitempty"`
}
