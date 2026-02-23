package utils

import (
	"fmt"
	"testing"
	"wireflow/management/model"
)

func TestGetJWTSecret(t *testing.T) {
	t.Run("should get secret", func(t *testing.T) {
		user := model.User{
			Email: "admin@123.com",
		}
		user.ID = "123"

		businessToken, err := GenerateBusinessJWT(user.ID, user.Email)
		if err != nil {
			t.Error(err)
		}

		s, err := ParseToken(businessToken)
		if err != nil {
			t.Error(err)
		}

		fmt.Println(s)
	})
}
