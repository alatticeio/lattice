package dto

import "linkany/management/utils"

type UserParams struct {
	Name string `json:"name" form:"name"`
}

func (up *UserParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if up.Name != "" {
		result = append(result, utils.NewKeyValue("name", up.Name))
	}

	return result
}
