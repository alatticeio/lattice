package server

import (
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) invitationRouter() {
	g := s.Group("/api/v1/workspaces/:id/invitations")
	g.Use(middleware.AuthMiddleware())
	{
		g.POST("", s.handleCreateInvitation())
		g.GET("", s.handleListInvitations())
		g.DELETE("/:invID", s.handleRevokeInvitation())
	}

	// Public invite endpoints — no auth required.
	pub := s.Group("/api/v1/invite")
	{
		// GET  /api/v1/invite/:token         — preview invitation info (public)
		pub.GET("/:token", s.handlePreviewInvitation())
		// POST /api/v1/invite/:token/register — register new account + accept (public)
		pub.POST("/:token/register", s.handleRegisterAndAccept())
		// POST /api/v1/invite/:token/accept   — accept with existing logged-in account
		pub.POST("/:token/accept", middleware.AuthMiddleware(), s.handleAcceptInvitation())
	}
}

func (s *Server) handleCreateInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		inviterID := c.GetString("user_id")

		var req struct {
			Email string            `json:"email" binding:"required,email"`
			Role  dto.WorkspaceRole `json:"role"  binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		inv, err := s.invitationController.Create(c.Request.Context(), wsID, inviterID, req.Email, req.Role)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, inv)
	}
}

func (s *Server) handleListInvitations() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		invs, err := s.invitationController.List(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, invs)
	}
}

func (s *Server) handleRevokeInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		invID := c.Param("invID")
		callerID := c.GetString("user_id")
		if err := s.invitationController.Revoke(c.Request.Context(), callerID, invID); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// handlePreviewInvitation is public — returns invitation metadata for display before login.
func (s *Server) handlePreviewInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		preview, err := s.invitationController.Preview(c.Request.Context(), token)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, preview)
	}
}

// handleRegisterAndAccept creates a new user account and accepts the invitation atomically.
func (s *Server) handleRegisterAndAccept() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required,min=6"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		jwtToken, err := s.invitationController.RegisterAndAccept(c.Request.Context(), token, req.Username, req.Password)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, gin.H{"token": jwtToken})
	}
}

func (s *Server) handleAcceptInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		userID := c.GetString("user_id")
		if err := s.invitationController.Accept(c.Request.Context(), token, userID); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
