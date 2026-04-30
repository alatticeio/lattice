package utils

import (
	"fmt"
	"testing"

	"github.com/alatticeio/lattice/internal/server/models"
)

func TestGetJWTSecret(t *testing.T) {
	t.Run("should get secret", func(t *testing.T) {
		user := models.User{
			Email: "admin@123.com",
		}
		user.ID = "123"

		businessToken, err := GenerateBusinessJWT(user.ID, user.Email, user.Username, "")
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
