package utils

import (
	"strconv"
	"strings"
)

func Splits(ids string, sep string) ([]uint, error) {
	if ids == "" {
		return nil, nil
	}
	idList := strings.Split(ids, sep)
	var list []uint
	for _, id := range idList {
		uid, err := StringToUint(id)
		if err != nil {
			return nil, err
		}
		list = append(list, uid)
	}
	return list, nil
}

func StringToUint(s string) (uint, error) {
	if s == "" {
		return 0, nil
	}

	result, err := strconv.Atoi(s)
	return uint(result), err
}
