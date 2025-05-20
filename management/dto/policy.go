package dto

import (
	"linkany/management/utils"
	"linkany/management/vo"
)

type AccessPolicyParams struct {
	vo.PageModel
	OwnId     *uint64 `json:"ownId" form:"ownId"`
	Name      string  `json:"name" form:"name"`
	GroupId   uint64  `json:"groupId" form:"groupId"`
	Effect    string  `json:"effect" form:"effect"`
	CreatedBy string  `json:"createdBy" form:"createdBy"`
	UpdatedBy string  `json:"updatedBy" form:"updatedBy"`
}

type AccessPolicyRuleParams struct {
	vo.PageModel
	Name       string `json:"name" form:"name"`
	OwnId      *uint64
	PolicyId   uint64 `json:"policyId" form:"policyId"`
	SourceId   string `json:"sourceId" form:"sourceId"`
	TargetId   string `json:"targetId" form:"targetId"`
	SourceType string `json:"sourceType" form:"sourceType"`
	TargetType string `json:"targetType" form:"targetType"`
}

func (p *AccessPolicyParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.OwnId != nil {
		result = append(result, utils.NewKeyValue("own_id", *p.OwnId))
	}

	if p.Name != "" {
		result = append(result, utils.NewKeyValue("name", p.Name))
	}

	if p.GroupId != 0 {
		result = append(result, utils.NewKeyValue("group_id", p.GroupId))
	}

	if p.Effect != "" {
		result = append(result, utils.NewKeyValue("effect", p.Effect))
	}

	if p.CreatedBy != "" {
		result = append(result, utils.NewKeyValue("created_by", p.CreatedBy))
	}

	if p.UpdatedBy != "" {
		result = append(result, utils.NewKeyValue("updated_by", p.UpdatedBy))
	}

	return result
}

func (p *AccessPolicyRuleParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.OwnId != nil {
		result = append(result, utils.NewKeyValue("own_id", *p.OwnId))
	}
	
	if p.PolicyId != 0 {
		result = append(result, utils.NewKeyValue("policy_id", p.PolicyId))
	}

	if p.SourceId != "" {
		result = append(result, utils.NewKeyValue("source_id", p.SourceId))
	}

	if p.TargetId != "" {
		result = append(result, utils.NewKeyValue("target_id", p.TargetId))
	}

	if p.SourceType != "" {
		result = append(result, utils.NewKeyValue("source_type", p.SourceType))
	}

	if p.TargetType != "" {
		result = append(result, utils.NewKeyValue("target_type", p.TargetType))
	}

	return result
}
