package entity

import (
	"gorm.io/gorm"
	"time"
)

// Invitation user invite other join its network
type Invitation struct {
	gorm.Model
	InvitationId int64 // invitation user id
	InviterId    int64 // inviter user id
	MobilePhone  string
	Email        string
	AcceptStatus AcceptStatus
	Network      string //192.168.0.0/24
	InvitedAt    time.Time
	AcceptAt     time.Time
}

func (i *Invitation) TableName() string {
	return "la_user_invitations"
}

type AcceptStatus int

const (
	NewInvite = iota
	Accept
	Rejected
)
