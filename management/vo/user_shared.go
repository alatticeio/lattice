package vo

import (
	"time"
)

type SharedNodeGroupVo struct {
	*GroupRelationVo
	ModelVo
	Name        string `json:"name"`
	GroupId     uint64 `json:"groupId"`
	InviteId    uint64 `json:"inviteId"`
	NodeCount   int    `json:"nodeCount"`
	Status      string `json:"status"`
	Description string `json:"description"`
	CreatedBy   string `json:"createdBy"`
	UpdatedBy   string `json:"updatedBy"`

	GroupNodes    []GroupNodeVo `json:"groupNodes"` // for tom-select show
	GroupPolicies []GroupPolicyVo
}

type SharedPolicyVo struct {
	ID          uint64    `json:"id"`
	UserId      uint64    `json:"userId"`
	PolicyId    uint64    `json:"policyId"`
	OwnerId     uint64    `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}

type SharedNodeVo struct {
	ID          uint64    `json:"id"`
	UserId      uint64    `json:"userId"`
	InviteId    uint64    `json:"inviteId"`
	NodeId      uint64    `json:"nodeId"`
	AppId       string    `json:"appId"`
	Address     *string   `json:"address"`
	Name        string    `json:"name"`
	OwnerId     uint64    `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`

	*LabelResourceVo
	NodeLabels []NodeLabelVo
}

type SharedLabelVo struct {
	ID          uint64    `json:"id"`
	UserId      uint64    `json:"userId"`
	LabelId     uint64    `json:"labelId"`
	LabelName   string    `json:"labelName"`
	OwnerId     uint64    `json:"ownerId"`
	Description string    `json:"description"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt"`
}
