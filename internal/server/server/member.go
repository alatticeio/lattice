package server

import (
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) memberRouter() {
	g := s.Group("/api/v1/workspaces/:id/members")
	g.Use(middleware.AuthMiddleware())
	{
		g.GET("", s.handleListMembers())
		g.POST("/:userID", s.handleAddMember())
		g.PUT("/:userID", s.handleUpdateMemberRole())
		g.DELETE("/:userID", s.handleRemoveMember())
	}
}

func (s *Server) handleListMembers() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		members, err := s.memberController.List(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, members)
	}
}

func (s *Server) handleAddMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		userID := c.Param("userID")

		if !s.requireWorkspaceAdmin(c, wsID) {
			c.Abort()
			return
		}

		var req struct {
			Role dto.WorkspaceRole `json:"role" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		// Check user is not already a member.
		if _, err := s.store.WorkspaceMembers().GetMembership(c.Request.Context(), wsID, userID); err == nil {
			resp.BadRequest(c, "user is already a member of this workspace")
			return
		}

		if err := s.memberController.Add(c.Request.Context(), wsID, userID, req.Role); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// requireWorkspaceAdmin checks that the caller is either a platform admin or a
// workspace admin in wsID. Returns false and writes the error response if not.
func (s *Server) requireWorkspaceAdmin(c *gin.Context, wsID string) bool {
	callerID := c.GetString("user_id")
	caller, err := s.store.Users().GetByID(c.Request.Context(), callerID)
	if err != nil {
		resp.Forbidden(c, "caller not found")
		return false
	}
	if caller.SystemRole == dto.SystemRolePlatformAdmin {
		return true
	}
	member, err := s.store.WorkspaceMembers().GetMembership(c.Request.Context(), wsID, callerID)
	if err != nil || dto.GetRoleWeight(member.Role) < dto.GetRoleWeight(dto.RoleAdmin) {
		resp.Forbidden(c, "only workspace admins can perform this action")
		return false
	}
	return true
}

func (s *Server) handleUpdateMemberRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		userID := c.Param("userID")

		if !s.requireWorkspaceAdmin(c, wsID) {
			c.Abort()
			return
		}

		var req struct {
			Role dto.WorkspaceRole `json:"role" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		// Prevent privilege escalation: cannot assign a role higher than your own.
		callerID := c.GetString("user_id")
		caller, _ := s.store.Users().GetByID(c.Request.Context(), callerID)
		if caller.SystemRole != dto.SystemRolePlatformAdmin {
			callerMember, err := s.store.WorkspaceMembers().GetMembership(c.Request.Context(), wsID, callerID)
			if err == nil && dto.GetRoleWeight(req.Role) > dto.GetRoleWeight(callerMember.Role) {
				resp.Forbidden(c, "cannot assign a role higher than your own")
				return
			}
		}

		if err := s.memberController.UpdateRole(c.Request.Context(), wsID, userID, req.Role); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func (s *Server) handleRemoveMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		userID := c.Param("userID")
		callerID := c.GetString("user_id")

		// Allow self-removal; otherwise require admin.
		if callerID != userID {
			if !s.requireWorkspaceAdmin(c, wsID) {
				c.Abort()
				return
			}
		}

		if err := s.memberController.Remove(c.Request.Context(), wsID, userID); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
