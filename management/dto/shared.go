package dto

import (
	"time"

	"gorm.io/gorm"
)

type SharedGroupDto struct {
	gorm.Model
	GroupId     uint   `json:"groupId"`
	InviteId    uint   `json:"inviteId"`
	NodeId      uint   `json:"nodeId"`
	PolicyId    uint   `json:"policyId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       uint64 `json:"ownerId"`
	IsPublic    bool   `json:"isPublic"`
	CreatedBy   string `json:"createdBy"`
	UpdatedBy   string `json:"updatedBy"`

	GroupRelationDto
}

type SharedPolicyDto struct {
	ID          uint      `json:"id"`
	UserId      uint      `json:"userId"`
	PolicyId    uint      `json:"policyId"`
	OwnerId     uint      `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}

type SharedNodeDto struct {
	ID          uint      `json:"id"`
	UserId      uint      `json:"userId"`
	NodeId      uint      `json:"nodeId"`
	OwnerId     uint      `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}

type SharedLabelDto struct {
	ID          uint      `json:"id"`
	UserId      uint      `json:"userId"`
	LabelId     uint      `json:"labelId"`
	OwnerId     uint      `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}
