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

func (s *Server) snapshotRouter() {
	ws := s.Group("/api/v1/workspaces/:id/snapshots")
	ws.Use(middleware.AuthMiddleware(nil))
	ws.GET("", s.handleListSnapshots())
	ws.GET("/:snapshotId", s.handleGetSnapshot())
	ws.GET("/diff", s.handleDiffSnapshots())

	ai := s.Group("/api/v1/ai")
	ai.Use(middleware.AuthMiddleware(nil))
	ai.POST("/debug", s.handleAIDebug())
}

// handleListSnapshots lists snapshots for a workspace.
// GET /api/v1/workspaces/:id/snapshots?from=...&to=...&triggerType=...
func (s *Server) handleListSnapshots() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		fromStr := c.DefaultQuery("from", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
		toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))
		triggerType := c.Query("triggerType")

		from, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			resp.BadRequest(c, "invalid from: "+err.Error())
			return
		}
		to, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			resp.BadRequest(c, "invalid to: "+err.Error())
			return
		}

		snaps, err := s.store.NetworkSnapshots().List(c.Request.Context(), wsID, from, to, triggerType)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, snaps)
	}
}

// handleGetSnapshot returns a single snapshot with full state.
// GET /api/v1/workspaces/:id/snapshots/:snapshotId
func (s *Server) handleGetSnapshot() gin.HandlerFunc {
	return func(c *gin.Context) {
		snapID := c.Param("snapshotId")
		snap, err := s.store.NetworkSnapshots().GetByID(c.Request.Context(), snapID)
		if err != nil {
			resp.Error(c, "snapshot not found")
			return
		}
		resp.OK(c, snap)
	}
}

// handleDiffSnapshots returns a diff between two snapshots.
// GET /api/v1/workspaces/:id/snapshots/diff?from=:id1&to=:id2
func (s *Server) handleDiffSnapshots() gin.HandlerFunc {
	return func(c *gin.Context) {
		fromID := c.Query("from")
		toID := c.Query("to")
		if fromID == "" || toID == "" {
			resp.BadRequest(c, "from and to snapshot IDs are required")
			return
		}
		fromSnap, err := s.store.NetworkSnapshots().GetByID(c.Request.Context(), fromID)
		if err != nil {
			resp.Error(c, "from snapshot not found")
			return
		}
		toSnap, err := s.store.NetworkSnapshots().GetByID(c.Request.Context(), toID)
		if err != nil {
			resp.Error(c, "to snapshot not found")
			return
		}
		resp.OK(c, gin.H{
			"from":      fromSnap,
			"to":        toSnap,
			"diffNotes": "Use the AI debug endpoint for human-readable root cause analysis.",
		})
	}
}

// handleAIDebug streams AI root cause analysis.
// POST /api/v1/ai/debug  body: { workspaceId, question, from, to }
func (s *Server) handleAIDebug() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.aiService == nil {
			resp.Error(c, "AI not configured")
			return
		}
		var req service.DebugRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		if req.WorkspaceID == "" || req.Question == "" {
			resp.BadRequest(c, "workspaceId and question are required")
			return
		}
		if req.To.IsZero() {
			req.To = time.Now()
		}
		if req.From.IsZero() {
			req.From = req.To.Add(-24 * time.Hour)
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		if err := s.aiService.Debug(c.Request.Context(), &req, &sseWriter{w: c.Writer}); err != nil {
			s.logger.Warn("AI debug error", "err", err)
		}
	}
}
