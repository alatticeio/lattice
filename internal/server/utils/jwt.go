// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package serverutils

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
