package entity

import (
	"gorm.io/gorm"
	"linkany/management/utils"
	"time"
)

// Invites invites invite others
type Invites struct {
	gorm.Model
	InvitationId int64 // invitation user id
	InviterId    int64 // inviter user id
	MobilePhone  string
	Email        string
	//Avatar       string
	GroupIds     string
	Group        string
	Role         string
	Permissions  string
	AcceptStatus AcceptStatus
	InvitedAt    time.Time
	CanceledAt   utils.NullTime
}

// Invitation user invite other join its network
type Invitation struct {
	gorm.Model
	InvitationId uint // invitation user id
	InviteeId    uint // inviter user id
	inviterName  string
	inviteeName  string
	AcceptStatus AcceptStatus //
	InviteId     uint         //relate to Invite table
	Group        string
	GroupIds     string
	Role         string
	Permissions  string
	Network      string //192.168.0.0/24
	InvitedAt    utils.NullTime
	AcceptAt     utils.NullTime
	RejectAt     utils.NullTime
}

func (i *Invites) TableName() string {
	return "la_user_invites"
}

func (i *Invitation) TableName() string {
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
