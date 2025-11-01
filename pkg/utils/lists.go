package utils

func Contains(sources []uint, target uint) bool {
	for _, v := range sources {
		if v == target {
			return true
		}
	}
	return false
}
