// Copyright 2026 The Lattice Authors, Inc.
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
	"github.com/alatticeio/lattice/internal/agent/infra"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mozillazg/go-pinyin" // handles Chinese transliteration to pinyin
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
	userId := ctx.Value(infra.UserIDKey)
	if userId == nil {
		return 0
	}

	return userId.(uint64)
}

func StringFormatter(a string) string {
	return strings.ToLower(a)
}

// GenerateAppId generates a unique application ID
// Format is similar to: lattice-20260116-a3f2
func GenerateAppId() string {
	// 1. Get the date part
	date := time.Now().Format("20060102")

	// 2. Generate 2 bytes (4 hex digits) of random data
	b := make([]byte, 2)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}

	randomPart := hex.EncodeToString(b)

	return fmt.Sprintf("lattice-%s-%s", date, randomPart)
}

func GenerateSlug(input string) string {
	// 1. If there is Chinese, convert to pinyin (optional — Chinese chars would be filtered by regex otherwise)
	args := pinyin.NewArgs()
	p := pinyin.Pinyin(input, args)
	if len(p) > 0 {
		var s []string
		for _, v := range p {
			s = append(s, v[0])
		}
		input = strings.Join(s, "-")
	}

	// 2. Convert to lowercase
	slug := strings.ToLower(input)

	// 3. Regex sanitization: keep only letters, digits, and hyphens
	reg, _ := regexp.Compile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")

	// 4. Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	return slug
}
