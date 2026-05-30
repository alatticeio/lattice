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
	"strings"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

// agentPolicyRouter registers AgentPolicy CRUD endpoints and the agent-facing
// MCP config endpoint (authenticated via agent JWT).
func (s *Server) agentPolicyRouter() {
	// Admin CRUD — standard user auth
	g := s.Group("/api/v1/agent-policies")
	g.Use(middleware.AuthMiddleware(s.revocationList))
	{
		g.GET("", s.handleListAgentPolicies())
		g.POST("", s.handleCreateAgentPolicy())
		g.GET("/:name", s.handleGetAgentPolicy())
		g.PUT("/:name", s.handleUpdateAgentPolicy())
		g.DELETE("/:name", s.handleDeleteAgentPolicy())
	}

	// Agent-facing: returns MCPServers + AgentPolicies for a registered agent.
	// Authenticated via Agent JWT (Bearer token from enrollment).
	// GET /api/v1/agent/mcp-config?namespace=<ns>
	s.GET("/api/v1/agent/mcp-config", s.handleAgentMCPConfig())
}

func (s *Server) handleListAgentPolicies() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		items, err := s.agentPolicySvc.List(c.Request.Context(), ns)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, items)
	}
}

func (s *Server) handleCreateAgentPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		var req struct {
			Name string                   `json:"name" binding:"required"`
			Spec v1alpha1.AgentPolicySpec `json:"spec" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		obj, err := s.agentPolicySvc.Create(c.Request.Context(), ns, req.Name, req.Spec)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleGetAgentPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		obj, err := s.agentPolicySvc.Get(c.Request.Context(), ns, c.Param("name"))
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleUpdateAgentPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		var req struct {
			Spec v1alpha1.AgentPolicySpec `json:"spec" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		obj, err := s.agentPolicySvc.Update(c.Request.Context(), ns, c.Param("name"), req.Spec)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleDeleteAgentPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		if err := s.agentPolicySvc.Delete(c.Request.Context(), ns, c.Param("name")); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// MCPConfigResponse is returned to sandbox agents to configure their MCP proxy.
type MCPConfigResponse struct {
	MCPServers    []v1alpha1.MCPServer   `json:"mcpServers"`
	AgentPolicies []v1alpha1.AgentPolicy `json:"agentPolicies"`
}

// handleAgentMCPConfig serves MCPServer + AgentPolicy config to a sandbox agent
// authenticated with its Agent JWT.
//
// GET /api/v1/agent/mcp-config?namespace=<ns>
// Authorization: Bearer <agent-jwt>
func (s *Server) handleAgentMCPConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentRegService == nil || s.mcpServerSvc == nil || s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "MCP config requires agent isolation and K8s")
			return
		}

		// Validate agent JWT from Authorization header.
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			resp.Unauthorized(c, "Bearer agent JWT required")
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := s.agentRegService.ValidateAgentJWT(token)
		if err != nil {
			resp.Unauthorized(c, "invalid agent JWT: "+err.Error())
			return
		}

		ns := claims.Namespace

		servers, err := s.mcpServerSvc.List(c.Request.Context(), ns)
		if err != nil {
			resp.Error(c, "list MCPServers: "+err.Error())
			return
		}

		policies, err := s.agentPolicySvc.List(c.Request.Context(), ns)
		if err != nil {
			resp.Error(c, "list AgentPolicies: "+err.Error())
			return
		}

		resp.OK(c, MCPConfigResponse{
			MCPServers:    servers,
			AgentPolicies: policies,
		})
	}
}
