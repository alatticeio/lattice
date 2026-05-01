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

// ParseToken 解析并校验 JWT
func ParseToken(tokenString string) (*models.LatticeClaims, error) {
	claims := &models.LatticeClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名算法: %v", token.Header["alg"])
		}

		secret := GetJWTSecret()
		return secret, nil
	})

	if err != nil {
		return nil, err
	}

	if token.Valid {
		return claims, nil
	}

	return nil, errors.New("token 验证失败：无效的凭证")
}

func GetJWTSecret() []byte {
	secret := os.Getenv("LATTICE_JWT_SECRET")
	if secret == "" {
		return []byte("your-256-bit-secret-key-here")
	}
	return []byte(secret)
}

func GenerateBusinessJWT(userID, email, username, systemRole string) (string, error) {
	claims := models.LatticeClaims{
		Email:      email,
		Username:   username,
		SystemRole: systemRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // jti for revocation tracking
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "lattice-bff",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(GetJWTSecret())
	if err != nil {
		return "", err
	}

	return signedToken, nil
}
