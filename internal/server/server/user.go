package server

import (
	"encoding/base64"
	"net/http"
	"time"

	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	serverutils "github.com/alatticeio/lattice/internal/server/utils"
	"github.com/alatticeio/lattice/pkg/utils"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

// generateRefreshToken creates a raw refresh token (returned to the caller)
// and persists its SHA256 hash in the database.
func (s *Server) generateRefreshToken(ctx *gin.Context, userID string) (string, error) {
	raw := make([]byte, 32)
	if err := utils.GenerateRandomBytes(raw); err != nil {
		return "", err
	}
	rawToken := base64.RawURLEncoding.EncodeToString(raw)

	now := time.Now()
	rt := &models.RefreshToken{
		UserID:    userID,
		TokenHash: models.HashRefreshToken(rawToken),
		ExpiresAt: now.Add(720 * time.Hour), // 30 days
	}
	if err := s.store.RefreshTokens().Create(ctx.Request.Context(), rt); err != nil {
		return "", err
	}
	return rawToken, nil
}

func (s *Server) userRouter() {

	userApi := s.Group("/api/v1/users")
	{
		userApi.POST("/register", ipLimiter.Middleware(10.0/60, 3), s.RegisterUser) // 10 req/min, burst 3
		userApi.POST("/login", ipLimiter.Middleware(5.0/60, 5), s.login)            // 5 req/min, burst 5
		userApi.GET("/getme", middleware.AuthMiddleware(s.revocationList), s.getMe())
		userApi.GET("/list", middleware.AuthMiddleware(s.revocationList), s.listUser())
		userApi.POST("/add", middleware.AuthMiddleware(s.revocationList), s.handleAddUser())
		userApi.DELETE("/:name", middleware.AuthMiddleware(s.revocationList), s.handleDeleteUser())
		userApi.PATCH("/:id/system-role", middleware.AuthMiddleware(s.revocationList), s.handleUpdateSystemRole())
		userApi.GET("/:id/workspaces", middleware.AuthMiddleware(s.revocationList), s.handleGetUserWorkspaces())
		userApi.PUT("/me/password", middleware.AuthMiddleware(s.revocationList), s.handleChangePassword())
	}
}

// RegisterUser handles user registration.
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

	c.JSON(http.StatusOK, gin.H{"message": "Registration successful"})
}

func (s *Server) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=1,max=128"`
		Client   string `json:"client"` // "cli" → long-lived token (30 days)
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, err.Error())
		return
	}

	user, err := s.userController.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	duration := 12 * time.Hour
	if req.Client == "cli" {
		duration = 720 * time.Hour // 30 days
	}

	businessToken, err := serverutils.GenerateBusinessJWTWithDuration(
		user.ID, user.Email, user.Username, string(user.SystemRole), duration,
	)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	// Issue refresh token (30-day, revocable)
	rawRefreshToken, err := s.generateRefreshToken(c, user.ID)
	if err != nil {
		resp.Error(c, "failed to issue refresh token")
		return
	}

	// Return to frontend
	resp.OK(c, map[string]interface{}{
		"user":         user.Username,
		"token":        businessToken,
		"refreshToken": rawRefreshToken,
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

		// Include tier from user profile so frontend can gate PRO features.
		tier := "community"
		if profile, pErr := s.store.Profiles().Get(c.Request.Context(), id); pErr == nil && profile.Tier != "" {
			tier = profile.Tier
		}

		resp.OK(c, map[string]any{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"avatar":     user.Avatar,
			"systemRole": user.SystemRole,
			"tier":       tier,
		})
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

// handleChangePassword allows the authenticated user to change their own password.
func (s *Server) handleChangePassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			resp.BadRequest(c, "unauthorized")
			return
		}

		var req struct {
			CurrentPassword string `json:"currentPassword" binding:"required"`
			NewPassword     string `json:"newPassword" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		ctx := c.Request.Context()
		user, err := s.store.Users().GetByID(ctx, userID)
		if err != nil {
			resp.Error(c, "user not found")
			return
		}

		if err = utils.ComparePassword(user.Password, req.CurrentPassword); err != nil {
			resp.BadRequest(c, "current password is incorrect")
			return
		}

		if err = utils.ValidatePassword(req.NewPassword); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		hashed, err := utils.EncryptPassword(req.NewPassword)
		if err != nil {
			resp.Error(c, "failed to update password")
			return
		}

		updated := &models.User{Password: hashed}
		updated.ID = userID
		if err = s.store.Users().Update(ctx, updated); err != nil {
			resp.Error(c, "failed to update password")
			return
		}

		resp.OK(c, nil)
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
