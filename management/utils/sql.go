package utils

import (
	"fmt"
	"strings"
)

// Generate will generate dynamic sql
func Generate(params ParamBuilder) (string, []interface{}) {
	var sb strings.Builder
	var wrappers []interface{}
	filters := params.Generate()
	for i, filter := range filters {
		if i < len(filters)-1 {
			sb.WriteString(fmt.Sprintf("%s = ? and ", filter.Key))
		} else {
			sb.WriteString(fmt.Sprintf("%s = ?", filter.Key))
		}
		wrappers = append(wrappers, filter.Value)
	}

	return sb.String(), wrappers
}

// GenerateSql  used for tom-select
func GenerateSql(params ParamBuilder) (string, []interface{}) {
	var sb strings.Builder
	var wrappers []interface{}
	filters := params.Generate()
	for i, filter := range filters {
		if i < len(filters)-1 {
			sb.WriteString(fmt.Sprintf("%s like ? and ", filter.Key))
		} else {
			sb.WriteString(fmt.Sprintf("%s like ?", filter.Key))
		}
		wrappers = append(wrappers, fmt.Sprintf("%%%v%%", filter.Value))
	}

	return sb.String(), wrappers
}
