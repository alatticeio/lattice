package server

import (
	"encoding/json"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) workflowRouter() {
	// Workspace-scoped workflow requests.
	ws := s.Group("/api/v1/workspaces/:id/workflow-requests")
	ws.Use(middleware.AuthMiddleware())
	{
		ws.GET("", s.handleListWorkflowRequests())
		ws.POST("", s.handleSubmitWorkflowRequest())
		ws.GET("/:reqId", s.handleGetWorkflowRequest())
		ws.POST("/:reqId/approve", s.handleApproveWorkflowRequest())
		ws.POST("/:reqId/reject", s.handleRejectWorkflowRequest())
	}

	// Platform-level workflow requests (platform admins).
	platform := s.Group("/api/v1/workflow-requests")
	platform.Use(middleware.AuthMiddleware())
	{
		platform.GET("", s.handleListWorkflowRequests())
		platform.GET("/:reqId", s.handleGetWorkflowRequest())
		platform.POST("/:reqId/approve", s.handleApproveWorkflowRequest())
		platform.POST("/:reqId/reject", s.handleRejectWorkflowRequest())
	}
}

func (s *Server) handleSubmitWorkflowRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")

		var body struct {
			ResourceType string          `json:"resourceType" binding:"required"`
			ResourceName string          `json:"resourceName"`
			Action       string          `json:"action" binding:"required"`
			Payload      json.RawMessage `json:"payload"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		payload := "{}"
		if body.Payload != nil {
			payload = string(body.Payload)
		}

		v, err := s.workflowController.Submit(c.Request.Context(), service.SubmitWorkflowReq{
			WorkspaceID:      wsID,
			RequestedBy:      c.GetString("user_id"),
			RequestedByName:  c.GetString("username"),
			RequestedByEmail: c.GetString("email"),
			ResourceType:     body.ResourceType,
			ResourceName:     body.ResourceName,
			Action:           body.Action,
			Payload:          payload,
		})
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		c.JSON(202, gin.H{"code": 0, "data": v})
	}
}

func (s *Server) handleListWorkflowRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		if wsID == "" {
			wsID = c.Query("workspaceId")
		}

		filter := store.WorkflowFilter{
			WorkspaceID:  wsID,
			ResourceType: c.Query("resourceType"),
			Action:       c.Query("action"),
			Status:       c.Query("status"),
		}
		if err := bindPage(c, &filter.Page, &filter.PageSize); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		result, err := s.workflowController.List(c.Request.Context(), filter)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, result)
	}
}

func (s *Server) handleGetWorkflowRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("reqId")
		if id == "" {
			resp.BadRequest(c, "request id is required")
			return
		}
		v, err := s.workflowController.GetByID(c.Request.Context(), id)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, v)
	}
}

func (s *Server) handleApproveWorkflowRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("reqId")
		var body struct {
			Note string `json:"note"`
		}
		_ = c.ShouldBindJSON(&body)

		err := s.workflowController.Approve(
			c.Request.Context(),
			id,
			c.GetString("user_id"),
			c.GetString("username"),
			body.Note,
		)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func (s *Server) handleRejectWorkflowRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("reqId")
		var body struct {
			Note string `json:"note"`
		}
		_ = c.ShouldBindJSON(&body)

		err := s.workflowController.Reject(
			c.Request.Context(),
			id,
			c.GetString("user_id"),
			c.GetString("username"),
			body.Note,
		)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
