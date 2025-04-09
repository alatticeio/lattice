package entity

import (
	"gorm.io/gorm"
	"linkany/management/utils"
	"time"
)

// InviteEntity invites invite others
type InviteEntity struct {
	gorm.Model
	InvitationId       int64 // invitation user id
	InvitationUsername string
	InviterId          int64 // inviter user id
	InviterUsername    string
	MobilePhone        string
	Email              string
	Avatar             string
	GroupIds           string
	Group              string
	Role               string
	Permissions        string
	AcceptStatus       AcceptStatus
	InvitedAt          time.Time
	CanceledAt         utils.NullTime

	// gorm Has Many
	SharedGroups      []SharedNodeGroup               `gorm:"foreignKey:InviteId"`
	SharedNodes       []SharedNode                    `gorm:"foreignKey:InviteId"`
	SharedPolicies    []SharedPolicy                  `gorm:"foreignKey:InviteId"`
	SharedLabels      []SharedLabel                   `gorm:"foreignKey:InviteId"`
	SharedPermissions []UserResourceGrantedPermission `gorm:"foreignKey:InviteId"`
}

// InvitationEntity user invite other join its network
type InvitationEntity struct {
	gorm.Model
	InvitationId uint // invitation user id
	InviteeId    uint // inviter user id
	//belongs to User
	User         User `gorm:"foreignKey:InviteeId"`
	inviterName  string
	inviteeName  string
	AcceptStatus AcceptStatus //
	InviteId     uint         //relate to InviteEntity table
	Group        string
	GroupIds     string
	Role         string
	Permissions  string
	Network      string //192.168.0.0/24
	InvitedAt    utils.NullTime
	AcceptAt     utils.NullTime
	RejectAt     utils.NullTime
}

func (i *InviteEntity) TableName() string {
	return "la_user_invites"
}

func (i *InvitationEntity) TableName() string {
	return "la_user_invitations"
}

type AcceptStatus int

const (
	NewInvite = iota
	Accept
	Rejected
	Canceled
)

func (a AcceptStatus) String() string {
	switch a {
	case NewInvite:
		return "pending"
	case Accept:
		return "accepted"
	case Rejected:
		return "rejected"
	case Canceled:
		return "canceled"
	default:
		return "unknown"
	}
}
