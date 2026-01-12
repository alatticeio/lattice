package utils

// GenericFilter 使用泛型 T，可以处理任何类型的切片
func Filter[T any](arr []T, predicate func(T) bool) []T {
	var result []T
	for _, item := range arr {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}
