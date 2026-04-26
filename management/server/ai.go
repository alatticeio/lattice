package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"wireflow/management/server/middleware"
	"wireflow/management/service"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) aiRouter() {
	if s.aiService == nil {
		// AI not configured: register stub handlers returning 503
		ai := s.Group("/api/v1/ai")
		ai.Use(middleware.AuthMiddleware())
		ai.POST("/chat", func(c *gin.Context) {
			resp.Error(c, "AI not configured: set ai.enabled=true and ai.api-key in wireflow.yaml")
		})
		ai.GET("/audit", func(c *gin.Context) {
			resp.Error(c, "AI not configured: set ai.enabled=true and ai.api-key in wireflow.yaml")
		})
		return
	}

	ai := s.Group("/api/v1/ai")
	ai.Use(middleware.AuthMiddleware())
	{
		ai.POST("/chat", s.handleAIChat())
		ai.GET("/audit", s.handleAIAudit())
	}
}

// handleAIChat streams an AI conversation response via Server-Sent Events.
//
// Request body:
//
//	{ "message": "...", "workspaceId": "ws-xxx", "history": [...] }
//
// Response: text/event-stream
//
//	data: {"type":"tool_use","tool":"list_peers","input":{}}
//	data: {"type":"token","content":"当前有 3 个 Peer..."}
//	data: {"type":"done"}
func (s *Server) handleAIChat() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req service.ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		if req.WorkspaceID == "" {
			resp.BadRequest(c, "workspaceId is required")
			return
		}
		if req.Message == "" {
			resp.BadRequest(c, "message is required")
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		w := &sseWriter{w: c.Writer}

		if err := s.aiService.Chat(c.Request.Context(), &req, w); err != nil {
			// Error already sent via SSE inside Chat(); nothing more to do.
			s.logger.Warn("AI chat error", "err", err)
		}
	}
}

// handleAIAudit runs a security audit on the workspace and returns findings.
//
// Query params: workspaceId=ws-xxx
func (s *Server) handleAIAudit() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Query("workspaceId")
		if wsID == "" {
			resp.BadRequest(c, "workspaceId is required")
			return
		}

		report, err := s.aiService.Audit(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		report.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
		resp.OK(c, report)
	}
}

// ── SSE writer ────────────────────────────────────────────────────────────────

// sseWriter implements service.StreamWriter and writes events in SSE format.
type sseWriter struct {
	w io.Writer
}

func (sw *sseWriter) Write(event service.StreamEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(sw.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	// Flush if the writer supports it
	if f, ok := sw.w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}
