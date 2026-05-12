package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) aiRouter() {
	ai := s.Group("/api/v1/ai")
	ai.Use(middleware.AuthMiddleware(nil))
	{
		ai.POST("/chat", s.handleAIChat())
		ai.GET("/audit", s.handleAIAudit())
		ai.GET("/tools", s.handleAIListTools())
		ai.POST("/tools/call", s.handleAIToolCall())
	}

	// Agent API routes — authenticated via Agent JWT issued by the server.
	// Agents call this endpoint to execute tools on behalf of their identity.
	if s.cfg.AI.AgentIsolation.Enabled && s.cfg.AI.AgentIsolation.JWTSecret != "" {
		agentAPI := s.Group("/api/v1/agents")
		agentAPI.Use(middleware.AgentAuth(s.cfg.AI.AgentIsolation.JWTSecret))
		{
			agentAPI.POST("/tools/call", s.handleAIToolCall())
		}
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

		ctx := c.Request.Context()
		if claims, ok := middleware.GetAgentClaims(c); ok {
			ctx = service.ContextWithAgentClaims(ctx, claims)
		}

		if err := s.aiService.Chat(ctx, &req, w); err != nil {
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

// handleAIListTools returns all available MCP tools for the workspace.
//
// Accepts workspaceId from: query param, X-Workspace-Id header, or context.
func (s *Server) handleAIListTools() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Query("workspaceId")
		if wsID == "" {
			wsID = c.GetHeader("X-Workspace-Id")
		}
		ns, err := s.resolveWorkspaceNamespace(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, "workspace not found: "+err.Error())
			return
		}
		tools := s.aiService.ListTools(ns)
		resp.OK(c, tools)
	}
}

// ToolCallRequest is the request body for POST /api/v1/ai/tools/call.
type ToolCallRequest struct {
	WorkspaceID string          `json:"workspaceId" binding:"required"`
	Tool        string          `json:"tool"        binding:"required"`
	Input       json.RawMessage `json:"input"`
}

// handleAIToolCall executes a single tool call and returns the result.
// Write tools may return a workflow approval ID instead of immediate results.
func (s *Server) handleAIToolCall() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ToolCallRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		wsID := req.WorkspaceID
		if wsID == "" {
			wsID = c.Query("workspaceId")
		}
		if wsID == "" {
			wsID = c.GetHeader("X-Workspace-Id")
		}
		ns, err := s.resolveWorkspaceNamespace(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, "workspace not found: "+err.Error())
			return
		}
		if req.Input == nil {
			req.Input = json.RawMessage(`{}`)
		}
		ctx := c.Request.Context()
		if claims, ok := middleware.GetAgentClaims(c); ok {
			ctx = service.ContextWithAgentClaims(ctx, claims)
		}
		result, err := s.aiService.ExecuteTool(ctx, ns, req.Tool, req.Input)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, gin.H{"result": result})
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
