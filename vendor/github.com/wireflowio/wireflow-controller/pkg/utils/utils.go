package utils

func Differences(old, new []string) (add, remove []string) {
	m := make(map[string]bool)
	for _, x := range new {
		m[x] = true
	}
	for _, x := range old {
		if _, found := m[x]; !found {
			remove = append(remove, x)
		} else {
			delete(m, x)
		}
	}
	for x := range m {
		add = append(add, x)
	}
	return
}

func RemoveStringFromSlice(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}

	return
}

func StringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 辅助函数
func StringSet(list []string) map[string]struct{} {
	set := make(map[string]struct{}, len(list))
	for _, item := range list {
		set[item] = struct{}{}
	}
	return set
}

func SetsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, exists := b[k]; !exists {
			return false
		}
	}
	return true
}
