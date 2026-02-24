package middleware

import (
	"context"
	"strings"
	"wireflow/internal/infra"
	"wireflow/pkg/utils"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取 Authorization Header (格式: Bearer <token>)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			resp.Unauthorized(c, "未授权，请先登录")
			c.Abort() // 停止执行后续的处理函数
			return
		}

		tokenString := authHeader[7:] // 截取 "Bearer " 之后的部分

		// 2. 解析并验证 Token
		claims, err := utils.ParseToken(tokenString) // 这里需要实现解析逻辑
		if err != nil {
			resp.Unauthorized(c, "无效的 Token")
			c.Abort()
			return
		}

		// 3. 将用户 ID 写入上下文，后续 Handler 可以通过 c.Get("userID") 拿到
		c.Set("user_id", claims.Subject)
		c.Set("name", claims.Name)
		c.Set("role", "admin")

		// 进阶：如果你想让后面的 context.Context 也能拿到这个值
		// 可以重写 Request 的 Context (可选，但在纯净的架构中很有用)
		ctx := context.WithValue(c.Request.Context(), infra.UserIDKey, claims.Subject)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
