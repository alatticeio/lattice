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
