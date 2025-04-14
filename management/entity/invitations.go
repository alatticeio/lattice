package entity

import (
	"linkany/management/utils"
	"time"
)

// InviterEntity invites invite others
type InviterEntity struct {
	Model
	InviteeId   uint64 // invitee user id
	InviterId   uint64 // inviter user id
	InviteeUser User   `gorm:"foreignKey:InviterId"`
	InviterUser User   `gorm:"foreignKey:InviterId"`
	//InviterUsername    string
	//InvitationUsername string
	//MobilePhone        string
	//Email              string
	//Avatar             string
	GroupIds     string
	Group        string
	Role         string
	Permissions  string
	AcceptStatus AcceptStatus
	InvitedAt    time.Time
	CanceledAt   utils.NullTime

	// gorm Has Many
	SharedGroups      []SharedNodeGroup               `gorm:"foreignKey:InviteId"`
	SharedNodes       []SharedNode                    `gorm:"foreignKey:InviteId"`
	SharedPolicies    []SharedPolicy                  `gorm:"foreignKey:InviteId"`
	SharedLabels      []SharedLabel                   `gorm:"foreignKey:InviteId"`
	SharedPermissions []UserResourceGrantedPermission `gorm:"foreignKey:InviteId"`
}

// InviteeEntity invitee data
type InviteeEntity struct {
	Model
	InviteeId uint64 // invitation user id
	InviterId uint64 // inviter user id
	//belongs to User
	User         User `gorm:"foreignKey:InviterId"`
	inviterName  string
	inviteeName  string
	AcceptStatus AcceptStatus //
	InviteId     uint64       //relate to InviterEntity table
	Group        string
	GroupIds     string
	Role         string
	Permissions  string
	Network      string //192.168.0.0/24
	InvitedAt    utils.NullTime
	AcceptAt     utils.NullTime
	RejectAt     utils.NullTime
}

func (i *InviterEntity) TableName() string {
	return "la_user_inviters"
}

func (i *InviteeEntity) TableName() string {
	return "la_user_invitees"
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
