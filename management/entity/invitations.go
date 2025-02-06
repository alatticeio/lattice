package entity

import (
	"gorm.io/gorm"
	"time"
)

// Invitations user invite other join its network
type Invitations struct {
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

func (i *Invitations) TableName() string {
	return "la_invitations"
}

type AcceptStatus int

const (
	NewInvite = iota
	Accept
	Rejected
)
