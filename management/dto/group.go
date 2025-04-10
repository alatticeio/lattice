package dto

import (
	"linkany/management/utils"
	"linkany/management/vo"
)

type GroupPolicyDto struct {
	ID          uint   `json:"id,string"`
	GroupId     uint   `json:"groupId,string"`
	PolicyId    uint   `json:"policyId,string"`
	PolicyName  string `json:"policyName"`
	Description string `json:"description"`
}

type GroupPolicyParams struct {
	GroupId    uint   `json:"groupId" form:"groupId"`
	PolicyId   uint   `json:"policyId" form:"policyId"`
	PolicyName string `json:"policyName" form:"policyName"`
}

type SharedGroupParams struct {
	UserId uint `json:"userId" form:"userId"`
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
