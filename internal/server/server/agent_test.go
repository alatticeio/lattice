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

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

// fakeAgentEnrollService is a test double for service.AgentEnrollService.
type fakeAgentEnrollService struct{}

func (f *fakeAgentEnrollService) Enroll(_ context.Context, req service.AgentEnrollRequest) (*service.AgentEnrollResponse, error) {
	return &service.AgentEnrollResponse{
		PeerName:        "agent-" + req.AgentName,
		EnrollmentToken: "lt-testtoken123",
		ExpiresAt:       time.Now().Add(time.Hour),
	}, nil
}

func (f *fakeAgentEnrollService) Revoke(_ context.Context, _, _ string) error {
	return nil
}

func setupAgentTestRouter(svc service.AgentEnrollService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/agent-enroll", handleAgentEnrollForTest(svc))
	r.DELETE("/api/v1/agent-enroll/:peerName", handleAgentRevokeForTest(svc))
	return r
}

func handleAgentEnrollForTest(svc service.AgentEnrollService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			AgentName    string `json:"agentName"    binding:"required"`
			AgentType    string `json:"agentType"    binding:"required"`
			WorkspaceID  string `json:"workspaceId"`
			TTLSeconds   int    `json:"ttlSeconds"`
			PolicyPreset string `json:"policyPreset"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.PolicyPreset == "" {
			req.PolicyPreset = service.PresetSandboxed
		}
		result, err := svc.Enroll(c.Request.Context(), service.AgentEnrollRequest{
			AgentName:    req.AgentName,
			AgentType:    req.AgentType,
			Namespace:    "test-ns",
			TTL:          time.Duration(req.TTLSeconds) * time.Second,
			PolicyPreset: req.PolicyPreset,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": result})
	}
}

func handleAgentRevokeForTest(svc service.AgentEnrollService) gin.HandlerFunc {
	return func(c *gin.Context) {
		peerName := c.Param("peerName")
		if err := svc.Revoke(c.Request.Context(), "test-ns", peerName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": nil})
	}
}

func TestAgentEnroll_ReturnsToken(t *testing.T) {
	g := NewWithT(t)
	r := setupAgentTestRouter(&fakeAgentEnrollService{})

	body, _ := json.Marshal(map[string]interface{}{
		"agentName":    "test-agent",
		"agentType":    "code-executor",
		"workspaceId":  "ws-test",
		"ttlSeconds":   3600,
		"policyPreset": "sandboxed",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-enroll", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	g.Expect(w.Code).To(Equal(http.StatusOK))
	var result struct {
		Data struct {
			EnrollmentToken string `json:"enrollmentToken"`
			PeerName        string `json:"peerName"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&result)
	g.Expect(result.Data.EnrollmentToken).To(HavePrefix("lt-"))
	g.Expect(result.Data.PeerName).To(Equal("agent-test-agent"))
}

func TestAgentRevoke_ReturnsOK(t *testing.T) {
	g := NewWithT(t)
	r := setupAgentTestRouter(&fakeAgentEnrollService{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agent-enroll/agent-test-agent?workspaceId=ws-test", nil)
	r.ServeHTTP(w, req)

	g.Expect(w.Code).To(Equal(http.StatusOK))
}
