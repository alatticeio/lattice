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
