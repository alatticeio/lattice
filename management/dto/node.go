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

	if l.PageNo == 0 {
		l.PageNo = PageNo
	}

	if l.PageSize == 0 {
		l.PageSize = PageSize
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
	GroupID *uint
	NodeId  *uint
	Role    *string
	Status  *int
}

func (p *GroupMemberParams) Generate() []*KeyValue {
	var result []*KeyValue

	if p.GroupID != nil {
		result = append(result, newKeyValue("group_id", p.GroupID))
	}

	if p.NodeId != nil {
		result = append(result, newKeyValue("node_id", p.NodeId))
	}

	if p.Role != nil {
		result = append(result, newKeyValue("role", p.Role))
	}

	if p.Status != nil {
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
