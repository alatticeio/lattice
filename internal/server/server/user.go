package server

import (
	"net/http"
	"time"

	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

func (s *Server) userRouter() {

	// Auth group — logout endpoint.
	authGroup := s.Group("/api/v1/auth")
	{
		authGroup.POST("/logout", middleware.AuthMiddleware(s.revocationList), s.logout())
	}

	userApi := s.Group("/api/v1/users")
	{
		userApi.POST("/register", s.RegisterUser)
		userApi.POST("/login", s.login)
		userApi.GET("/getme", middleware.AuthMiddleware(s.revocationList), s.getMe())
		userApi.GET("/list", middleware.AuthMiddleware(s.revocationList), s.listUser())
		userApi.POST("/add", middleware.AuthMiddleware(s.revocationList), s.handleAddUser())
		userApi.DELETE("/:name", middleware.AuthMiddleware(s.revocationList), s.handleDeleteUser())
		userApi.PATCH("/:id/system-role", middleware.AuthMiddleware(s.revocationList), s.handleUpdateSystemRole())
		userApi.GET("/:id/workspaces", middleware.AuthMiddleware(s.revocationList), s.handleGetUserWorkspaces())
	}
}

// 用户注册
func (s *Server) RegisterUser(c *gin.Context) {
	var req dto.UserDto
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	err := s.userController.Register(ctx, req)

	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "注册成功"})
}

func (s *Server) login(c *gin.Context) {
	var req dto.UserDto
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, err.Error())
		return
	}

	user, err := s.userController.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	businessToken, err := utils.GenerateBusinessJWT(user.ID, user.Email, user.Username, string(user.SystemRole))
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	// 返回给前端
	resp.OK(c, map[string]interface{}{
		"user":  user.Username,
		"token": businessToken,
	})
}

func (s *Server) getMe() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetString("user_id")
		if id == "" {
			resp.BadRequest(c, `"ext_id" is empty`)
			return
		}

		user, err := s.userController.GetMe(c.Request.Context(), id)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, user)
	}
}

func (s *Server) handleAddUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.UserDto
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		err := s.userController.AddUser(c.Request.Context(), &req)

		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, nil)
	}
}

func (s *Server) handleDeleteUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			resp.BadRequest(c, `"name" is empty`)
			return
		}

		err := s.userController.DeleteUser(c.Request.Context(), name)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, nil)
	}
}

func (s *Server) handleUpdateSystemRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only platform_admin can change system roles.
		if c.GetString("system_role") != "platform_admin" {
			resp.Error(c, "forbidden: platform_admin only")
			return
		}

		userID := c.Param("id")
		if userID == "" {
			resp.BadRequest(c, "user id is required")
			return
		}

		var body struct {
			SystemRole string `json:"systemRole" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		role := dto.SystemRole(body.SystemRole)
		if err := s.userController.UpdateSystemRole(c.Request.Context(), userID, role); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// listUser list members
func (s *Server) listUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.PageRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		res, err := s.userController.ListUser(c.Request.Context(), &req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, res)
	}
}

// handleGetUserWorkspaces returns all workspace memberships for a given user.
func (s *Server) handleGetUserWorkspaces() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("id")
		if userID == "" {
			resp.BadRequest(c, "user id is required")
			return
		}

		ctx := c.Request.Context()

		members, _, err := s.store.WorkspaceMembers().ListByUser(ctx, userID, 1, 999)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		type wsEntry struct {
			WorkspaceID   string `json:"workspaceId"`
			WorkspaceName string `json:"workspaceName"`
			Role          string `json:"role"`
		}

		result := make([]wsEntry, 0, len(members))
		for _, m := range members {
			ws, err := s.store.Workspaces().GetByID(ctx, m.WorkspaceID)
			if err != nil {
				continue
			}
			result = append(result, wsEntry{
				WorkspaceID:   m.WorkspaceID,
				WorkspaceName: ws.DisplayName,
				Role:          string(m.Role),
			})
		}

		resp.OK(c, result)
	}
}

func (s *Server) logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		jti := c.GetString("jti")
		if jti == "" {
			resp.Error(c, "invalid token")
			return
		}

		expRaw, exists := c.Get("exp")
		if !exists {
			s.revocationList.Revoke(jti, time.Now().Add(12*time.Hour))
		} else {
			if exp, ok := expRaw.(time.Time); ok {
				s.revocationList.Revoke(jti, exp)
			} else {
				s.revocationList.Revoke(jti, time.Now().Add(12*time.Hour))
			}
		}

		resp.OK(c, nil)
	}
}
