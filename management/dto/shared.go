package dto

import (
	"time"
)

type SharedGroupDto struct {
	ID          uint64
	GroupId     uint64 `json:"groupId"`
	InviteId    uint64 `json:"inviteId"`
	NodeId      uint64 `json:"nodeId"`
	PolicyId    uint64 `json:"policyId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       uint64 `json:"ownerId"`
	IsPublic    bool   `json:"isPublic"`
	CreatedBy   string `json:"createdBy"`
	UpdatedBy   string `json:"updatedBy"`

	GroupRelationDto
}

type SharedPolicyDto struct {
	ID          uint64    `json:"id"`
	UserId      uint64    `json:"userId"`
	PolicyId    uint64    `json:"policyId"`
	OwnerId     uint64    `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}

type SharedNodeDto struct {
	ID          uint64    `json:"id"`
	UserId      uint64    `json:"userId"`
	NodeId      uint64    `json:"nodeId"`
	OwnerId     uint64    `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}

type SharedLabelDto struct {
	ID          uint64    `json:"id"`
	UserId      uint64    `json:"userId"`
	LabelId     uint64    `json:"labelId"`
	OwnerId     uint64    `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}
