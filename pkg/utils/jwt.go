package utils

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/alatticeio/lattice/internal/server/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func ParseToken(tokenString string) (*models.LatticeClaims, error) {
	claims := &models.LatticeClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing algorithm: %v", token.Header["alg"])
		}
		return GetJWTSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if token.Valid {
		return claims, nil
	}
	return nil, errors.New("token verification failed: invalid credentials")
}

func GetJWTSecret() []byte {
	secret := os.Getenv("LATTICE_JWT_SECRET")
	if secret == "" {
		return []byte("your-256-bit-secret-key-here")
	}
	return []byte(secret)
}

// GenerateBusinessJWT issues a short-lived JWT (12h) for Dashboard sessions.
func GenerateBusinessJWT(userID, email, username, systemRole string) (string, error) {
	return GenerateBusinessJWTWithDuration(userID, email, username, systemRole, 12*time.Hour)
}

// GenerateBusinessJWTWithDuration issues a JWT with an explicit lifetime.
func GenerateBusinessJWTWithDuration(userID, email, username, systemRole string, duration time.Duration) (string, error) {
	claims := models.LatticeClaims{
		Email:      email,
		Username:   username,
		SystemRole: systemRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "lattice-bff",
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}
