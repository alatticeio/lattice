package dex

import (
	"strings"

	"github.com/alatticeio/lattice/internal/server/models"
	serverutils "github.com/alatticeio/lattice/internal/server/utils"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Define keys used in Context
const (
	CtxUserKey      = "userID"
	CtxTeamKey      = "teamID"
	CtxNamespaceKey = "namespace"
)

// AuthMiddleware Gin authentication middleware
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Get Authorization Header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			resp.Unauthorized(c, "Authorization header is missing or invalid")
			c.Abort() // Must call Abort to prevent subsequent handlers from executing
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 2. Parse JWT
		claims := models.LatticeClaims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
			return serverutils.GetJWTSecret(), nil
		})

		// 3. Validate token
		if err != nil || !token.Valid {
			resp.Unauthorized(c, "Token is expired or invalid")
			c.Abort()
			return
		}

		// 4. Inject key info into Gin Context
		// This way subsequent route handlers can retrieve it directly via c.GetString("namespace")
		c.Set(CtxUserKey, claims.Subject)

		c.Next() // Continue to subsequent handlers
	}
}
