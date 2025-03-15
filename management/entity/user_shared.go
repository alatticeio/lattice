package entity

import (
	"linkany/management/utils"
	"time"

	"gorm.io/gorm"
)

// SharedGroup is the entity that represents the shared group
type SharedGroup struct {
	gorm.Model
	UserId       uint
	GroupId      uint
	OwnerId      uint `gorm:"column:owner_id;size:20" json:"ownerId"`
	AcceptStatus AcceptStatus
	Description  string
	GrantedAt    utils.NullTime
	RevokedAt    utils.NullTime
}

// TableName returns the table name of the shared group
func (SharedGroup) TableName() string {
	return "la_shared_group"
}

// SharedPolicy is the entity that represents the shared policy
type SharedPolicy struct {
	gorm.Model
	UserId       uint
	PolicyId     uint
	OwnerId      uint
	Description  string
	AcceptStatus AcceptStatus
	GrantedAt    utils.NullTime
	RevokedAt    utils.NullTime
}

// TableName returns the table name of the shared policy
func (SharedPolicy) TableName() string {
	return "la_shared_policy"
}

// SharedNode is the entity that represents the shared node
type SharedNode struct {
	gorm.Model
	UserId       uint
	NodeId       uint
	OwnerId      uint
	AcceptStatus AcceptStatus
	Description  string
	GrantedAt    utils.NullTime
	RevokedAt    time.Time
}

func (SharedNode) TableName() string {
	return "la_shared_node"
}

type SharedLabel struct {
	gorm.Model
	UserId       uint
	LabelId      uint
	OwnerId      uint
	AcceptStatus AcceptStatus
	Description  string
	GrantedAt    utils.NullTime
	RevokedAt    utils.NullTime
}

func (SharedLabel) TableName() string {
	return "la_shared_label"
}
