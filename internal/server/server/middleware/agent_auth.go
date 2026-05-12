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

package middleware

import (
	"net/http"
	"strings"

	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const agentClaimsKey = "agent_claims"

// AgentAuth is a Gin middleware that validates Agent JWTs and injects
// AgentClaims into the context. Returns 401 if the token is missing or invalid.
func AgentAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing agent token"})
			return
		}
		raw := strings.TrimPrefix(authHeader, "Bearer ")

		claims := &models.AgentClaims{}
		_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
			return
		}

		c.Set(agentClaimsKey, claims)
		c.Next()
	}
}

// GetAgentClaims extracts AgentClaims from the Gin context.
// Returns (nil, false) if no agent claims are present (e.g., user token path).
func GetAgentClaims(c *gin.Context) (*models.AgentClaims, bool) {
	v, exists := c.Get(agentClaimsKey)
	if !exists {
		return nil, false
	}
	claims, ok := v.(*models.AgentClaims)
	return claims, ok
}
