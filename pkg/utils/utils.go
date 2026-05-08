package utils

import (
	"crypto/sha256"
	"fmt"
)

// DeriveNamespace derives a namespace name from a token
func DeriveNamespace(token string) string {
	h := sha256.Sum256([]byte(token))
	// Take the first 12 hex chars of the hash, produce a name like wf-a1b2c3d4e5f6
	return fmt.Sprintf("wf-%x", h[:6])
}
