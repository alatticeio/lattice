package middleware

import (
	"context"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func TenantContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, _ := c.Get("role")
		wsID := c.GetHeader("X-Workspace-Id")

		// 1. 超级管理员：拥有“选择性隔离”的特权
		if userRole == "admin" {
			if wsID != "" {
				injectWorkspace(c, wsID, false) // 管理员不需要强制校验归属关系
			}
			c.Next()
			return
		}

		// 2. 普通用户：必须提供 wsID
		if wsID == "" {
			resp.Forbidden(c, "请先选择工作空间")
			c.Abort()
			return
		}

		// 3. 核心安全逻辑：此处应有校验（建议查缓存或鉴权服务）
		// 确保该用户真的属于这个 Workspace，防止 ID 遍历攻击
		// if !checkUserInWorkspace(c.GetUint("user_id"), wsID) { ... }

		// 4. 普通用户强制进入“严格模式”
		injectWorkspace(c, wsID, true)
		c.Next()
	}
}

func injectWorkspace(c *gin.Context, wsID string, strict bool) {
	// 注入 workspaceId
	ctx := context.WithValue(c.Request.Context(), "workspaceId", wsID)
	// 注入是否严格过滤的标记
	ctx = context.WithValue(ctx, "isStrictTenant", strict)
	c.Request = c.Request.WithContext(ctx)
}
