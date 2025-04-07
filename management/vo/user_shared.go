package vo

import (
	"time"
)

type SharedNodeGroupVo struct {
	*GroupRelationVo
	ModelVo
	Name        string `json:"name"`
	NodeCount   int    `json:"nodeCount"`
	Status      string `json:"status"`
	Description string `json:"description"`
	CreatedBy   string `json:"createdBy"`
	UpdatedBy   string `json:"updatedBy"`

	GroupNodes    []GroupNodeVo `json:"groupNodes"` // for tom-select show
	GroupPolicies []GroupPolicyVo
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
