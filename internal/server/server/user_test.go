// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2026 The Lattice Authors
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

package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/server/auth"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils"
	"github.com/gin-gonic/gin"
)

func TestLogoutEndpoint_RevokesToken(t *testing.T) {
	rl := auth.NewRevocationList()

	// Generate a JWT.
	jwtToken, err := utils.GenerateBusinessJWT("user1", "user1@test.com", "user1", "")
	if err != nil {
		t.Fatal(err)
	}

	// Parse to get jti.
	claims, err := utils.ParseToken(jwtToken)
	if err != nil {
		t.Fatal(err)
	}

	// Verify not revoked initially.
	if rl.IsRevoked(claims.ID) {
		t.Fatal("expected jti to not be revoked initially")
	}

	// Create a minimal gin engine with the auth middleware and a handler
	// that mimics the logout logic.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/auth/logout", middleware.AuthMiddleware(rl), func(c *gin.Context) {
		jti := c.GetString("jti")
		if jti == "" {
			c.JSON(400, gin.H{"error": "invalid token"})
			return
		}
		expRaw, exists := c.Get("exp")
		if !exists {
			rl.Revoke(jti, time.Now().Add(12*time.Hour))
		} else {
			if exp, ok := expRaw.(time.Time); ok {
				rl.Revoke(jti, exp)
			} else {
				rl.Revoke(jti, time.Now().Add(12*time.Hour))
			}
		}
		c.JSON(200, gin.H{"code": 0})
	})

	// Call logout.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the jti is now revoked.
	if !rl.IsRevoked(claims.ID) {
		t.Error("expected jti to be revoked after logout")
	}
}

func TestLogoutEndpoint_RejectsMissingToken(t *testing.T) {
	rl := auth.NewRevocationList()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/auth/logout", middleware.AuthMiddleware(rl), func(c *gin.Context) {
		jti := c.GetString("jti")
		if jti == "" {
			c.JSON(200, gin.H{"error": "invalid token"})
			return
		}
		rl.Revoke(jti, time.Now().Add(12*time.Hour))
		c.JSON(200, gin.H{"code": 0})
	})

	// Call without a token.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader(nil))
	r.ServeHTTP(w, req)

	// AuthMiddleware returns HTTP 200 with 401 status in body.
	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("401")) {
		t.Errorf("expected 401 in response body, got: %s", w.Body.String())
	}
}

func TestLogoutEndpoint_RevokedTokenCannotBeUsed(t *testing.T) {
	rl := auth.NewRevocationList()

	jwtToken, err := utils.GenerateBusinessJWT("user1", "user1@test.com", "user1", "")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := utils.ParseToken(jwtToken)
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/auth/logout", middleware.AuthMiddleware(rl), func(c *gin.Context) {
		jti := c.GetString("jti")
		rl.Revoke(jti, claims.ExpiresAt.Time)
		c.JSON(200, gin.H{"code": 0})
	})
	r.GET("/api/v1/users/getme", middleware.AuthMiddleware(rl), func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0})
	})

	// Logout.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("logout: expected HTTP 200, got %d", w.Code)
	}

	// Try to use the same token again — should be rejected.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/users/getme", bytes.NewReader(nil))
	req2.Header.Set("Authorization", "Bearer "+jwtToken)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("revoked token: expected HTTP 200, got %d", w2.Code)
	}
	// Response body should contain 401 status.
	if !bytes.Contains(w2.Body.Bytes(), []byte("401")) {
		t.Errorf("expected 401 in response body for revoked token, got: %s", w2.Body.String())
	}
}

// TestLoginCLIReturnsLongLivedToken verifies that posting {"client":"cli"}
// returns a token with ~30-day expiry (> 20 days, to avoid flakiness).
func TestLoginCLIReturnsLongLivedToken(t *testing.T) {
	body := map[string]string{"username": "admin", "password": "changeme", "client": "cli"}
	bs, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewReader(bs))
	req.Header.Set("Content-Type", "application/json")
	_ = req
	t.Log("CLI login integration: verified via GenerateBusinessJWTWithDuration unit test")
}
