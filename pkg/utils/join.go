// Copyright 2025 Wireflow.io, Inc.
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
	"fmt"
	"strings"
)

// Join 接收任意类型的切片和分隔符，返回连接后的字符串
func Join[T any](arr []T, sep string) string {
	if len(arr) == 0 {
		return ""
	}

	// 创建字符串构建器
	var builder strings.Builder

	// 处理第一个元素
	builder.WriteString(fmt.Sprint(arr[0]))

	// 处理剩余元素
	for _, item := range arr[1:] {
		builder.WriteString(sep)
		builder.WriteString(fmt.Sprint(item))
	}

	return builder.String()
}

func String2Array[T any](s string, sep string) []string {
	return strings.Split(s, sep)
}
