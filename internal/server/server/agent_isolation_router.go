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

package server

import (
	"context"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// agentIsolationRouter registers the Agent Isolation admin API.
// All routes require standard user authentication.
// Routes return 402 when the feature is disabled (agentRegService == nil).
func (s *Server) agentIsolationRouter() {
	g := s.Group("/api/v1/agent-isolation")
	g.Use(middleware.AuthMiddleware(s.revocationList))
	{
		g.POST("/enrollment-tokens", s.handleCreateEnrollmentToken())
		g.POST("/register", s.handleAgentRegister())
		g.DELETE("/agents/:name", s.handleIsolationAgentRevoke())
	}
}

// handleCreateEnrollmentToken creates a one-time enrollment token for an agent.
//
// POST /api/v1/agent-isolation/enrollment-tokens
// Body: { "namespace": "default", "allowedTools": ["list_peers"], "ttlSeconds": 3600 }
func (s *Server) handleCreateEnrollmentToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentRegService == nil {
			resp.PaymentRequired(c, "agent isolation is not enabled")
			return
		}
		var req struct {
			Namespace    string   `json:"namespace"    binding:"required"`
			AllowedTools []string `json:"allowedTools"`
			TTLSeconds   int      `json:"ttlSeconds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		ttl := time.Duration(req.TTLSeconds) * time.Second
		result, err := s.agentRegService.CreateEnrollmentToken(c.Request.Context(), service.EnrollmentTokenRequest{
			AllowedNamespace: req.Namespace,
			AllowedTools:     req.AllowedTools,
			TTL:              ttl,
			CreatedBy:        c.GetString("user_id"),
		})
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, result)
	}
}

// handleAgentRegister exchanges an enrollment token for an Agent JWT.
//
// POST /api/v1/agent-isolation/register
// Body: { "enrollmentToken": "...", "agentName": "claude-1", "publicKey": "wg-pubkey", "sandbox": "gvisor" }
func (s *Server) handleAgentRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentRegService == nil {
			resp.PaymentRequired(c, "agent isolation is not enabled")
			return
		}
		var req struct {
			EnrollmentToken string `json:"enrollmentToken" binding:"required"`
			AgentName       string `json:"agentName"       binding:"required"`
			PublicKey       string `json:"publicKey"       binding:"required"`
			Sandbox         string `json:"sandbox"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		result, err := s.agentRegService.RegisterAgent(c.Request.Context(), service.AgentRegisterRequest{
			EnrollmentToken: req.EnrollmentToken,
			AgentName:       req.AgentName,
			PublicKey:       req.PublicKey,
			Sandbox:         v1alpha1.SandboxMode(req.Sandbox),
		})
		if err != nil {
			msg := err.Error()
			if msg == "invalid enrollment token" || msg == "enrollment token already used" || msg == "enrollment token expired" {
				resp.BadRequest(c, msg)
			} else {
				resp.Error(c, msg)
			}
			return
		}
		resp.OK(c, result)
	}
}

// handleIsolationAgentRevoke revokes an agent by deleting its AgentIdentity CRD.
//
// DELETE /api/v1/agent-isolation/agents/:name?namespace=<ns>
func (s *Server) handleIsolationAgentRevoke() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentRegService == nil {
			resp.PaymentRequired(c, "agent isolation is not enabled")
			return
		}
		name := c.Param("name")
		namespace := c.Query("namespace")
		if namespace == "" {
			resp.BadRequest(c, "namespace query param is required")
			return
		}
		if err := s.agentRegService.RevokeAgent(c.Request.Context(), namespace, name); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// ── k8sAgentIdentityReader ────────────────────────────────────────────────────

// k8sAgentIdentityReader adapts a controller-runtime client.Client to
// service.AgentIdentityReader.
type k8sAgentIdentityReader struct {
	c client.Client
}

func (r *k8sAgentIdentityReader) Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentIdentity, error) {
	identity := &v1alpha1.AgentIdentity{}
	if err := r.c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, identity); err != nil {
		return nil, err
	}
	return identity, nil
}
