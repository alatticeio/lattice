package server

import (
	"errors"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

var (
	errRefreshTokenRequired = errors.New("refreshToken is required")
	errRefreshFailed        = errors.New("refresh failed")
	errLogoutFailed         = errors.New("logout failed")
)

func (s *Server) authRouter(authService service.AuthService) {
	auth := s.Group("/api/v1/auth")
	{
		auth.POST("/refresh", s.handleRefreshToken(authService))
		auth.POST("/logout", s.handleLogout(authService))
	}
}

func (s *Server) handleRefreshToken(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refreshToken" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, errRefreshTokenRequired.Error())
			return
		}

		accessToken, newRefreshToken, err := authService.RefreshAccessToken(c.Request.Context(), req.RefreshToken)
		if err != nil {
			// Return safe errors to the client; log internal errors.
			msg := safeRefreshError(err)
			if msg == errRefreshFailed.Error() {
				log.GetLogger("auth-handler").Error("refresh failed", err)
			}
			resp.Error(c, msg)
			return
		}

		resp.OK(c, map[string]interface{}{
			"token":        accessToken,
			"refreshToken": newRefreshToken,
		})
	}
}

func (s *Server) handleLogout(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refreshToken" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, errRefreshTokenRequired.Error())
			return
		}

		if err := authService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
			msg := safeLogoutError(err)
			if msg == errLogoutFailed.Error() {
				log.GetLogger("auth-handler").Error("logout failed", err)
			}
			resp.Error(c, msg)
			return
		}

		resp.OK(c, nil)
	}
}

// safeRefreshError maps known error strings to safe client-facing messages.
// Unknown/internal errors are replaced with a generic "refresh failed".
func safeRefreshError(err error) string {
	switch err.Error() {
	case "invalid refresh token",
		"refresh token has been revoked",
		"refresh token has expired",
		"user not found",
		"user account is disabled":
		return err.Error()
	default:
		return errRefreshFailed.Error()
	}
}

// safeLogoutError maps known error strings to safe client-facing messages.
func safeLogoutError(err error) string {
	switch err.Error() {
	case "invalid refresh token":
		return err.Error()
	default:
		return errLogoutFailed.Error()
	}
}
