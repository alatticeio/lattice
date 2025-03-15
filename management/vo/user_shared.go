package vo

import "time"

type SharedGroupVo struct {
	ID          uint      `json:"id"`
	UserId      uint      `json:"userId"`
	GroupId     uint      `json:"groupId"`
	OwnerID     uint      `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}

type SharedPolicyVo struct {
	ID          uint      `json:"id"`
	UserId      uint      `json:"userId"`
	PolicyId    uint      `json:"policyId"`
	OwnerId     uint      `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}

type SharedNodeVo struct {
	ID          uint      `json:"id"`
	UserId      uint      `json:"userId"`
	NodeId      uint      `json:"nodeId"`
	OwnerId     uint      `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}

type SharedLabelVo struct {
	ID          uint      `json:"id"`
	UserId      uint      `json:"userId"`
	LabelId     uint      `json:"labelId"`
	OwnerId     uint      `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}
