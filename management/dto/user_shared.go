package dto

import "time"

type SharedGroupDto struct {
	ID          uint      `json:"id"`
	UserId      uint      `json:"userId"`
	GroupId     uint      `json:"groupId"`
	OwnerID     uint      `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
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
