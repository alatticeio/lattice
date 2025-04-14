package utils

import (
	"context"
	"github.com/google/uuid"
	"strconv"
	"strings"
)

func Splits(ids string, sep string) ([]uint64, error) {
	if ids == "" {
		return nil, nil
	}
	idList := strings.Split(ids, sep)
	var list []uint64
	for _, id := range idList {
		uid, err := StringToUint64(id)
		if err != nil {
			return nil, err
		}
		list = append(list, uid)
	}
	return list, nil
}

func StringToUint64(s string) (uint64, error) {
	if s == "" {
		return 0, nil
	}

	result, err := strconv.ParseUint(s, 10, 64)
	return result, err
}

func GenerateUUID() string {
	uuid := uuid.New()
	return strings.ReplaceAll(uuid.String(), "-", "")
}

func GetUserIdFromCtx(ctx context.Context) uint64 {
	userId := ctx.Value("userId")
	if userId == nil {
		return 0
	}

	return userId.(uint64)
}
