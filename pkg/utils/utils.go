package utils

import (
	"crypto/sha256"
	"fmt"
)

// 示例：通过 Token 派生 Namespace 名称
func DeriveNamespace(token string) string {
	h := sha256.Sum256([]byte(token))
	// 取哈希的前 12 位，生成类似 wf-a1b2c3d4e5f6 的名字
	return fmt.Sprintf("wf-%x", h[:6])
}
