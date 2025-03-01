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
