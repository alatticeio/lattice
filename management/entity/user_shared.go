package entity

import (
	"linkany/management/utils"

	"gorm.io/gorm"
)

// SharedNodeGroup is the entity that represents the shared group
type SharedNodeGroup struct {
	gorm.Model
	UserId       uint
	GroupId      uint
	GroupName    string
	OwnerId      uint `gorm:"column:owner_id;size:20" json:"ownerId"`
	InviteId     uint
	AcceptStatus AcceptStatus
	Description  string
	GrantedAt    utils.NullTime
	RevokedAt    utils.NullTime

	GroupNodes    []GroupNode   `gorm:"foreignKey:GroupId;references:GroupId"`
	GroupPolicies []GroupPolicy `gorm:"foreignKey:GroupId;references:GroupId"`
}

// TableName returns the table name of the shared group
func (SharedNodeGroup) TableName() string {
	return "la_shared_group"
}

// SharedPolicy is the entity that represents the shared policy
type SharedPolicy struct {
	gorm.Model
	UserId       uint
	PolicyId     uint
	PolicyName   string
	OwnerId      uint
	InviteId     uint
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
	Node         Node `gorm:"foreignKey:NodeId"`
	NodeName     string
	OwnerId      uint
	InviteId     uint
	AcceptStatus AcceptStatus
	Description  string
	GrantedAt    utils.NullTime
	RevokedAt    utils.NullTime

	NodeLabels []NodeLabel `gorm:"foreignKey:NodeId;references:NodeId"`
}

func (SharedNode) TableName() string {
	return "la_shared_node"
}

type SharedLabel struct {
	gorm.Model
	UserId       uint
	LabelId      uint
	LabelName    string
	OwnerId      uint
	InviteId     uint
	AcceptStatus AcceptStatus
	Description  string
	GrantedAt    utils.NullTime
	RevokedAt    utils.NullTime
}

func (SharedLabel) TableName() string {
	return "la_shared_label"
}
