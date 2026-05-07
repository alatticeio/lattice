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
	"time"

	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

func (s *Server) agentRouter() {
	g := s.Group("/api/v1")
	g.Use(middleware.AuthMiddleware(s.revocationList))
	g.POST("/agent-enroll", s.handleAgentEnroll())
	g.DELETE("/agent-enroll/:peerName", s.handleAgentRevoke())
}

type agentEnrollRequest struct {
	AgentName    string `json:"agentName"    binding:"required"`
	AgentType    string `json:"agentType"    binding:"required"`
	WorkspaceID  string `json:"workspaceId"  binding:"required"`
	TTLSeconds   int    `json:"ttlSeconds"`   // 0 = no expiry
	PolicyPreset string `json:"policyPreset"` // default: sandboxed
}

// handleAgentEnroll creates a new agent peer with WireGuard identity.
//
//	POST /api/v1/agent-enroll
//	body: { agentName, agentType, workspaceId, ttlSeconds, policyPreset }
func (s *Server) handleAgentEnroll() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req agentEnrollRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		if req.PolicyPreset == "" {
			req.PolicyPreset = service.PresetSandboxed
		}

		ws, err := s.store.Workspaces().GetByID(c.Request.Context(), req.WorkspaceID)
		if err != nil {
			resp.Error(c, "workspace not found: "+err.Error())
			return
		}

		ttl := time.Duration(req.TTLSeconds) * time.Second
		result, err := s.agentEnrollService.Enroll(c.Request.Context(), service.AgentEnrollRequest{
			AgentName:    req.AgentName,
			AgentType:    req.AgentType,
			Namespace:    ws.Namespace,
			TTL:          ttl,
			PolicyPreset: req.PolicyPreset,
		})
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, result)
	}
}

// handleAgentRevoke deletes an agent peer immediately (before TTL expiry).
//
//	DELETE /api/v1/agent-enroll/:peerName?workspaceId=ws-xxx
func (s *Server) handleAgentRevoke() gin.HandlerFunc {
	return func(c *gin.Context) {
		peerName := c.Param("peerName")
		wsID := c.Query("workspaceId")
		if wsID == "" {
			resp.BadRequest(c, "workspaceId is required")
			return
		}
		ws, err := s.store.Workspaces().GetByID(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, "workspace not found")
			return
		}
		if err := s.agentEnrollService.Revoke(c.Request.Context(), ws.Namespace, peerName); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
