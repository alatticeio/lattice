package utils

import (
	"testing"

	"github.com/alatticeio/lattice/internal/server/models"
)

func TestGetJWTSecret(t *testing.T) {
	t.Run("should generate and parse JWT with jti", func(t *testing.T) {
		user := models.User{
			Email: "admin@123.com",
		}
		user.ID = "123"

		businessToken, err := GenerateBusinessJWT(user.ID, user.Email, user.Username, "")
		if err != nil {
			t.Fatal(err)
		}

		claims, err := ParseToken(businessToken)
		if err != nil {
			t.Fatal(err)
		}

		if claims.ID == "" {
			t.Error("expected jti (ID) to be non-empty")
		}
		if claims.Subject != "123" {
			t.Errorf("expected sub=123, got %s", claims.Subject)
		}
	})
}
