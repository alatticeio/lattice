package middleware

import (
	"context"
	"strings"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/auth"
	"github.com/alatticeio/lattice/pkg/utils"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(revocationList *auth.RevocationList) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Get Authorization Header (format: Bearer <token>)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			resp.Unauthorized(c, "Unauthorized, please log in first")
			c.Abort() // Stop executing subsequent handlers
			return
		}

		tokenString := authHeader[7:] // Extract everything after "Bearer "

		// 2. Parse and validate the token
		claims, err := utils.ParseToken(tokenString) // Parsing logic needs to be implemented here
		if err != nil {
			resp.Unauthorized(c, "Invalid token")
			c.Abort()
			return
		}

		// 3. Check revocation (skip if list is nil — for migration during Task 5)
		if revocationList != nil && claims.ID != "" && revocationList.IsRevoked(claims.ID) {
			resp.Unauthorized(c, "token has been revoked")
			c.Abort()
			return
		}

		// 4. Write user ID to context; subsequent handlers can retrieve it via c.Get("userID")
		c.Set("user_id", claims.Subject)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("system_role", claims.SystemRole)
		c.Set("jti", claims.ID)
		c.Set("exp", claims.ExpiresAt.Time)

		// Advanced: if you want the context.Context downstream to also carry this value,
		// you can rewrite the Request's Context (optional, but useful in a clean architecture)
		ctx := context.WithValue(c.Request.Context(), infra.UserIDKey, claims.Subject)
		ctx = context.WithValue(ctx, infra.SystemRoleKey, claims.SystemRole)
		ctx = context.WithValue(ctx, infra.UsernameKey, claims.Username)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
