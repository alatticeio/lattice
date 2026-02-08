package dex

import (
	"net/http"
	"strings"
	"wireflow/management/model"
	"wireflow/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// 定义 Context 中使用的 Key
const (
	CtxUserKey      = "userID"
	CtxTeamKey      = "teamID"
	CtxNamespaceKey = "namespace"
)

// AuthMiddleware Gin 鉴权中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取 Authorization Header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is missing or invalid"})
			c.Abort() // 必须调用 Abort 阻止后续 Handler 执行
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 2. 解析 JWT
		claims := model.WireFlowClaims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
			return utils.GetJWTSecret(), nil
		})

		// 3. 校验 Token 有效性
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token is expired or invalid"})
			c.Abort()
			return
		}

		// 4. 关键信息注入 Gin Context
		// 这样后续的路由 Handler 就可以通过 c.GetString("namespace") 直接拿到了
		c.Set(CtxUserKey, claims.Subject)
		c.Set(CtxTeamKey, claims.TeamID)

		c.Next() // 继续执行后续流程
	}
}
