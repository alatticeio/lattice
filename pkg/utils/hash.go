package utils

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

func Hash(obj interface{}) string {
	data, err := json.Marshal(obj)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%x", sha256.Sum256(data))
}
