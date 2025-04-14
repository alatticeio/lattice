package dto

import (
	"linkany/management/utils"
	"linkany/management/vo"
)

type LabelParams struct {
	vo.PageModel
	CreatedBy string
	UpdatedBy string
}

func (l *LabelParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if l.CreatedBy != "" {
		result = append(result, utils.NewKeyValue("created_by", l.CreatedBy))
	}

	if l.UpdatedBy != "" {
		result = append(result, utils.NewKeyValue("updated_by", l.UpdatedBy))
	}

	if l.Page == 0 {
		l.Page = utils.PageNo
	}

	if l.Size == 0 {
		l.Size = utils.PageSize
	}

	if l.Current == 0 {
		l.Current = utils.PageNo
	}

	return result
}

type GroupParams struct {
	vo.PageModel
	Name        string
	Description string
	OwnerID     uint
	IsPublic    *bool
}

func (p *GroupParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.Name != "" {
		result = append(result, utils.NewKeyValue("name", p.Name))
	}

	if p.Description != "" {
		result = append(result, utils.NewKeyValue("description", p.Description))
	}

	if p.OwnerID != 0 {
		result = append(result, utils.NewKeyValue("owner_id", p.OwnerID))
	}

	if p.IsPublic != nil {
		result = append(result, utils.NewKeyValue("is_public", p.IsPublic))
	}

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}

	return result
}

type GroupMemberParams struct {
	vo.PageModel
	GroupID   uint   `json:"groupID"`
	UserID    uint   `json:"userID"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	GroupName string `json:"groupName"`
	Username  string `json:"username"`
}

func (p *GroupMemberParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.GroupID != 0 {
		result = append(result, utils.NewKeyValue("group_id", p.GroupID))
	}

	if p.UserID != 0 {
		result = append(result, utils.NewKeyValue("user_id", p.UserID))
	}

	if p.GroupName != "" {
		result = append(result, utils.NewKeyValue("group_name", p.GroupName))
	}

	if p.Username != "" {
		result = append(result, utils.NewKeyValue("username", p.Username))
	}

	if p.Role != "" {
		result = append(result, utils.NewKeyValue("role", p.Role))
	}

	if p.Status != "" {
		result = append(result, utils.NewKeyValue("status", p.Status))
	}

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}

	return result
}

type GroupNodeParams struct {
	vo.PageModel
	GroupID   uint64 `json:"groupID"`
	NodeId    uint64 `json:"nodeId"`
	GroupName string `json:"groupName"`
	NodeName  string `json:"nodeName"`
	CreatedBy string `json:"createdBy"`
}

func (p *GroupNodeParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.GroupID != 0 {
		result = append(result, utils.NewKeyValue("group_id", p.GroupID))
	}

	if p.GroupName != "" {
		result = append(result, utils.NewKeyValue("group_name", p.GroupName))
	}

	if p.NodeId != 0 {
		result = append(result, utils.NewKeyValue("node_id", p.NodeId))
	}

	if p.CreatedBy != "" {
		result = append(result, utils.NewKeyValue("created_by", p.CreatedBy))
	}

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}

	return result
}

type NodeLabelParams struct {
	vo.PageModel
	NodeId    uint64 `json:"nodeId"`
	LabelId   uint64 `json:"labelId"`
	Label     string `json:"name"`
	CreatedBy string `json:"createdBy"`
}

func (p *NodeLabelParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.NodeId != 0 {
		result = append(result, utils.NewKeyValue("node_id", p.NodeId))
	}

	if p.LabelId != 0 {
		result = append(result, utils.NewKeyValue("label_id", p.LabelId))
	}

	if p.Label != "" {
		result = append(result, utils.NewKeyValue("label_name", p.Label))
	}

	if p.CreatedBy != "" {
		result = append(result, utils.NewKeyValue("created_by", p.CreatedBy))
	}

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}

	return result
}
