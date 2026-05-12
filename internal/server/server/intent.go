package server

import (
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) intentRouter() {
	if s.intentService == nil {
		// Intent engine not available: return 503
		intent := s.Group("/api/v1/ai/intent")
		intent.Use(middleware.AuthMiddleware(nil))
		intent.POST("/plan", func(c *gin.Context) {
			resp.Error(c, "network intent engine is not available (AI not configured)")
		})
		intent.POST("/apply", func(c *gin.Context) {
			resp.Error(c, "network intent engine is not available (AI not configured)")
		})
		return
	}

	intent := s.Group("/api/v1/ai/intent")
	intent.Use(middleware.AuthMiddleware(nil))
	{
		intent.POST("/plan", s.handleIntentPlan())
		intent.POST("/apply", s.handleIntentApply())
		intent.GET("/history", s.handleIntentHistory())
	}
}

type intentPlanRequest struct {
	WorkspaceID string `json:"workspaceId" binding:"required"`
	Intent      string `json:"intent"      binding:"required"`
	DryRun      bool   `json:"dryRun"`
}

// handleIntentPlan generates a change plan from a natural-language intent.
//
//	POST /api/v1/ai/intent/plan
//	body: { workspaceId, intent, dryRun }
func (s *Server) handleIntentPlan() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req intentPlanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		ws, err := s.store.Workspaces().GetByID(c.Request.Context(), req.WorkspaceID)
		if err != nil {
			resp.Error(c, "workspace not found")
			return
		}
		plan, err := s.intentService.Plan(c.Request.Context(), service.IntentRequest{
			WorkspaceID: req.WorkspaceID,
			Namespace:   ws.Namespace,
			Intent:      req.Intent,
			DryRun:      req.DryRun,
		})
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, plan)
	}
}

type intentApplyRequest struct {
	PlanID string `json:"planId" binding:"required"`
}

// handleIntentHistory returns the last N applied plans for a workspace.
//
//	GET /api/v1/ai/intent/history?workspaceId=...
func (s *Server) handleIntentHistory() gin.HandlerFunc {
	return func(c *gin.Context) {
		workspaceID := c.Query("workspaceId")
		if workspaceID == "" {
			resp.BadRequest(c, "workspaceId is required")
			return
		}
		items, err := s.intentService.History(c.Request.Context(), workspaceID, 20)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, items)
	}
}

// handleIntentApply executes a previously generated plan via WorkflowService.
//
//	POST /api/v1/ai/intent/apply
//	body: { planId }
func (s *Server) handleIntentApply() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req intentApplyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		userID := c.GetString("user_id")
		workflowIDs, err := s.intentService.Apply(c.Request.Context(), req.PlanID, userID)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, gin.H{"workflowIds": workflowIDs, "message": "Plan has been submitted for approval; it will be executed automatically upon approval"})
	}
}
