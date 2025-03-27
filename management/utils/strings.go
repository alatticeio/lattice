package utils

import (
	"context"
	"github.com/google/uuid"
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

func GenerateUUID() string {
	uuid := uuid.New()
	return strings.ReplaceAll(uuid.String(), "-", "")
}

func GetUserIdFromCtx(ctx context.Context) uint {
	userId := ctx.Value("userId")
	if userId == nil {
		return 0
	}

	return userId.(uint)
}
