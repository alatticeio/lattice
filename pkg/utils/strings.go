// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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

func StringFormatter(a string) string {
	return strings.ToLower(a)
}

// generateAppId 生成一个唯一的程序 ID
// 格式类似于: wire-20260116-a3f2
func GenerateAppId() string {
	// 1. 取得日期部分
	date := time.Now().Format("20060102")

	// 2. 生成 2 字节（4位十六进制）的随机数
	b := make([]byte, 2)
	rand.Read(b)
	randomPart := hex.EncodeToString(b)

	return fmt.Sprintf("wireflow-%s-%s", date, randomPart)
}
