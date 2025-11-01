package utils

// GenerateQuery for building query conditions
func GenerateQuery(params ParamBuilder, like bool) *QueryConditions {
	conditions := NewQueryConditions()
	filters := params.Generate()
	for _, filter := range filters {
		if like {
			conditions.AddLike(filter.Key, filter.Value)
		} else {
			conditions.AddWhere(filter.Key, filter.Value)
		}
	}

	return conditions
}
