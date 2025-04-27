package dto

import (
	"linkany/management/utils"
	"linkany/management/vo"
)

type Params interface {
	Generate() []*utils.KeyValue
}

type GroupPolicyDto struct {
	ID          uint64 `json:"id,string"`
	GroupId     uint64 `json:"groupId,string"`
	PolicyId    uint64 `json:"policyId,string"`
	PolicyName  string `json:"policyName"`
	Description string `json:"description"`
}

type GroupPolicyParams struct {
	vo.PageModel
	GroupId    uint64 `json:"groupId" form:"groupId"`
	PolicyId   uint64 `json:"policyId" form:"policyId"`
	PolicyName string `json:"policyName" form:"policyName"`
}

func (g *GroupPolicyParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	return result
}

type SharedGroupParams struct {
	ID      uint64
	GroupId uint64 `json:"groupId" form:"groupId"`
	UserId  uint64 `json:"userId" form:"userId"`
	GroupParams
	InviteId uint64
}

type SharedPolicyParams struct {
	vo.PageModel
	InviteId *uint64
	PolicyId *uint64
}

type SharedNodeParams struct {
	vo.PageModel
	InviteId *uint64
	NodeId   *uint64
}

type SharedLabelParams struct {
	vo.PageModel
	InviteId *uint64
	LabelId  *uint64
}

func (p *SharedNodeParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue
	if p.InviteId != nil {
		result = append(result, utils.NewKeyValue("invite_id", p.InviteId))
	}

	if p.NodeId != nil {
		result = append(result, utils.NewKeyValue("node_id", p.NodeId))
	}

	return result
}

func (p *SharedPolicyParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue
	if p.InviteId != nil {
		result = append(result, utils.NewKeyValue("invite_id", p.InviteId))
	}

	if p.PolicyId != nil {
		result = append(result, utils.NewKeyValue("policy_id", p.PolicyId))
	}

	return result
}

func (p *SharedLabelParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue
	if p.InviteId != nil {
		result = append(result, utils.NewKeyValue("invite_id", p.InviteId))
	}

	if p.LabelId != nil {
		result = append(result, utils.NewKeyValue("label_id", p.LabelId))
	}

	return result
}
