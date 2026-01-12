package service

import (
	"testing"
	"wireflow/internal/config"
	"wireflow/pkg/utils"
)

func TestToken(t *testing.T) {
	t.Run("TestToken", func(t *testing.T) {
		token, err := GenerateSecureToken()
		if err != nil {
			t.Fatal(err)
		}
		t.Log(token)

		encoded := config.StringToBase64(token)
		t.Log(encoded)

		nsName := utils.DeriveNamespace(token)
		t.Log(nsName)

		res := string([]byte(token))
		t.Log(res)

		// bzhKZU9YYjN1Qmg1cHU4bA==
		decoded, err := config.Base64Decode("bzhKZU9YYjN1Qmg1cHU4bA==")
		if err != nil {
			t.Fatal(err)
		}

		t.Log(string(decoded))

		nsName = utils.DeriveNamespace(string(decoded))
		t.Log(nsName)

	})
}
