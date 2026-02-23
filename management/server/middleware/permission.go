package middleware

import (
	"net/http"
	"wireflow/management/dto"
	"wireflow/management/service"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

type Middleware struct {
	workspaceMemberService service.WorkspaceMemberService
}

// WorkspaceAuthMiddleware 权限拦截器
func (m *Middleware) WorkspaceAuthMiddleware(requiredRole dto.WorkspaceRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从 Context 获取 UserID (通常由之前的 JWT 中间件解析并存入)
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			c.Abort()
			return
		}

		// 2. 获取请求路径或 Header 中的 WorkspaceID
		workspaceId := c.Param("workspaceId") // 或者 c.GetHeader("X-Workspace-ID")
		if workspaceId == "" {
			resp.BadRequest(c, "workspace id is required")
			c.Abort()
			return
		}

		// 3. 数据库查询：校验 WorkspaceMember 关系
		member, err := m.workspaceMemberService.GetMemberRole(c.Request.Context(), workspaceId, userID.(string))
		if err != nil {
			resp.Forbidden(c, "你不是该团队成员")
			c.Abort()
			return
		}

		if dto.GetRoleWeight(member) < dto.GetRoleWeight(requiredRole) {
			resp.Forbidden(c, "权限不足")
			c.Abort()
			return
		}
		// 5. 校验通过，把当前成员信息存入上下文，方便后续使用
		c.Set("currentTeamMember", member)
		c.Next()
	}
}
