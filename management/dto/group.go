package dto

import (
	"linkany/management/utils"
	"linkany/management/vo"
)

type GroupPolicyDto struct {
	ID          uint64 `json:"id,string"`
	GroupId     uint64 `json:"groupId,string"`
	PolicyId    uint64 `json:"policyId,string"`
	PolicyName  string `json:"policyName"`
	Description string `json:"description"`
}

type GroupPolicyParams struct {
	GroupId    uint64 `json:"groupId" form:"groupId"`
	PolicyId   uint64 `json:"policyId" form:"policyId"`
	PolicyName string `json:"policyName" form:"policyName"`
}

type SharedGroupParams struct {
	UserId uint64 `json:"userId" form:"userId"`
	GroupParams
}

type SharedPolicyParams struct {
	vo.PageModel
}

type SharedNodeParams struct {
	vo.PageModel
}

type SharedLabelParams struct {
	vo.PageModel
}

func (p *SharedNodeParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}

	return result
}

func (p *SharedPolicyParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}

	return result
}

func (p *SharedLabelParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}

	return result
}
