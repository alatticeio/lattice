package utils

import (
	"fmt"
	"testing"
)

func TestGetPublicKey(t *testing.T) {
	for i := 0; i < 2; i++ {
		key, err := GetPublicKey()
		if err != nil {
			t.Fatal(err)
		}

		fmt.Println(key)
	}
}
