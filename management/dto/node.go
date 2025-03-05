package dto

type LabelParams struct {
	PageModel
	CreatedBy string
	UpdatedBy string
}

func (l *LabelParams) Generate() []*KeyValue {
	var result []*KeyValue

	if l.CreatedBy != "" {
		result = append(result, newKeyValue("created_by", l.CreatedBy))
	}

	if l.UpdatedBy != "" {
		result = append(result, newKeyValue("updated_by", l.UpdatedBy))
	}

	if l.Page == 0 {
		l.Page = PageNo
	}

	if l.Size == 0 {
		l.Size = PageSize
	}

	if l.Current == 0 {
		l.Current = PageNo
	}

	return result
}

type GroupParams struct {
	PageModel
	Name        *string
	Description *string
	OwnerID     *uint
	IsPublic    *bool
}

func (p *GroupParams) Generate() []*KeyValue {
	var result []*KeyValue

	if p.Name != nil {
		result = append(result, newKeyValue("name", p.Name))
	}

	if p.Description != nil {
		result = append(result, newKeyValue("description", p.Description))
	}

	if p.OwnerID != nil {
		result = append(result, newKeyValue("owner_id", p.OwnerID))
	}

	if p.IsPublic != nil {
		result = append(result, newKeyValue("is_public", p.IsPublic))
	}

	if p.PageNo == 0 {
		p.PageNo = PageNo
	}

	if p.PageSize == 0 {
		p.PageSize = PageSize
	}

	return result
}

type GroupMemberParams struct {
	PageModel
	GroupID   uint   `json:"groupID"`
	UserID    uint   `json:"userID"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	GroupName string `json:"groupName"`
	Username  string `json:"username"`
}

func (p *GroupMemberParams) Generate() []*KeyValue {
	var result []*KeyValue

	if p.GroupID != 0 {
		result = append(result, newKeyValue("group_id", p.GroupID))
	}

	if p.UserID != 0 {
		result = append(result, newKeyValue("user_id", p.UserID))
	}

	if p.GroupName != "" {
		result = append(result, newKeyValue("group_name", p.GroupName))
	}

	if p.Username != "" {
		result = append(result, newKeyValue("username", p.Username))
	}

	if p.Role != "" {
		result = append(result, newKeyValue("role", p.Role))
	}

	if p.Status != "" {
		result = append(result, newKeyValue("status", p.Status))
	}

	if p.PageNo == 0 {
		p.PageNo = PageNo
	}

	if p.PageSize == 0 {
		p.PageSize = PageSize
	}

	return result
}

type GroupNodeParams struct {
	PageModel
	GroupID   uint   `json:"groupID"`
	NodeId    uint   `json:"nodeId"`
	GroupName string `json:"groupName"`
	NodeName  string `json:"nodeName"`
	CreatedBy string `json:"createdBy"`
}

func (p *GroupNodeParams) Generate() []*KeyValue {
	var result []*KeyValue

	if p.GroupID != 0 {
		result = append(result, newKeyValue("group_id", p.GroupID))
	}

	if p.GroupName != "" {
		result = append(result, newKeyValue("group_name", p.GroupName))
	}

	if p.NodeId != 0 {
		result = append(result, newKeyValue("node_id", p.NodeId))
	}

	if p.CreatedBy != "" {
		result = append(result, newKeyValue("created_by", p.CreatedBy))
	}

	if p.PageNo == 0 {
		p.PageNo = PageNo
	}

	if p.PageSize == 0 {
		p.PageSize = PageSize
	}

	return result
}
