package utils

import (
	"fmt"
	"linkany/management/dto"
	"reflect"
	"strings"
)

// Generate will generate dynamic sql
func Generate(params dto.ParamBuilder) (string, []interface{}) {
	var sb strings.Builder
	var wrappers []interface{}
	filters := params.Generate()
	for i, filter := range filters {
		if i < len(filters)-1 {
			sb.WriteString(fmt.Sprintf("%s = ? and ", filter.Key))
		} else {
			sb.WriteString(fmt.Sprintf("%s = ?", filter.Key))
		}
		wrappers = append(wrappers, reflect.ValueOf(filter.Value).Elem().Interface())
	}

	return sb.String(), wrappers
}
