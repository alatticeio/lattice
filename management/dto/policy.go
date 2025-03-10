package dto

import "linkany/management/vo"

type AccessPolicyParams struct {
	*vo.PageModel
	Name      string `json:"name" form:"name"`
	GroupId   uint64 `json:"groupId" form:"groupId"`
	Effect    string `json:"effect" form:"effect"`
	CreatedBy string `json:"createdBy" form:"createdBy"`
	UpdatedBy string `json:"updatedBy" form:"updatedBy"`
}

type AccessPolicyRuleParams struct {
	vo.PageModel
	PolicyId   int64  `json:"policyId" form:"policyId"`
	SourceId   string `json:"sourceId" form:"sourceId"`
	TargetId   string `json:"targetId" form:"targetId"`
	SourceType string `json:"sourceType" form:"sourceType"`
	TargetType string `json:"targetType" form:"targetType"`
}

func (p *AccessPolicyParams) Generate() []*KeyValue {
	var result []*KeyValue

	if p.Name != "" {
		result = append(result, newKeyValue("name", p.Name))
	}

	if p.GroupId != 0 {
		result = append(result, newKeyValue("group_id", p.GroupId))
	}

	if p.Effect != "" {
		result = append(result, newKeyValue("effect", p.Effect))
	}

	if p.CreatedBy != "" {
		result = append(result, newKeyValue("created_by", p.CreatedBy))
	}

	if p.UpdatedBy != "" {
		result = append(result, newKeyValue("updated_by", p.UpdatedBy))
	}

	if p.Page == 0 {
		p.Page = PageNo
	}

	if p.Size == 0 {
		p.Size = PageSize
	}

	return result
}

func (p *AccessPolicyRuleParams) Generate() []*KeyValue {
	var result []*KeyValue

	if p.PolicyId != 0 {
		result = append(result, newKeyValue("policy_id", p.PolicyId))
	}

	if p.SourceId != "" {
		result = append(result, newKeyValue("source_id", p.SourceId))
	}

	if p.TargetId != "" {
		result = append(result, newKeyValue("target_id", p.TargetId))
	}

	if p.SourceType != "" {
		result = append(result, newKeyValue("source_type", p.SourceType))
	}

	if p.TargetType != "" {
		result = append(result, newKeyValue("target_type", p.TargetType))
	}

	if p.Page == 0 {
		p.Page = PageNo
	}

	if p.Size == 0 {
		p.Size = PageSize
	}

	return result
}
