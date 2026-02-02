package middleware

import (
	"strings"
	"wireflow/pkg/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取 Authorization Header (格式: Bearer <token>)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(401, gin.H{"error": "未授权，请先登录"})
			c.Abort() // 停止执行后续的处理函数
			return
		}

		tokenString := authHeader[7:] // 截取 "Bearer " 之后的部分

		// 2. 解析并验证 Token
		claims, err := utils.ParseToken(tokenString) // 这里需要实现解析逻辑
		if err != nil {
			c.JSON(401, gin.H{"error": "无效的 Token"})
			c.Abort()
			return
		}

		// 3. 将用户 ID 写入上下文，后续 Handler 可以通过 c.Get("userID") 拿到
		c.Set("userID", claims.UserID)
		c.Next()
	}
}
