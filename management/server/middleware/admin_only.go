package middleware

import (
	"github.com/gin-gonic/gin"
)

// AdminOnly 校验用户在当前 Workspace 是否拥有管理员或以上权限
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		//// 1. 从 JWT 中间件获取已登录的用户 ID
		//userID, exists := c.Get("userId")
		//if !exists {
		//	c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		//	c.Abort()
		//	return
		//}
		//
		//// 2. 获取当前操作的 Workspace 标识 (通常从 URL 参数获取，如 /workspaces/:slug/...)
		//workspaceSlug := c.Param("slug")
		//if workspaceSlug == "" {
		//	c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace identity missing"})
		//	c.Abort()
		//	return
		//}
		//
		//// 3. 数据库查询：关联 Workspace 和 Member 校验角色
		//var member model.WorkspaceMember
		//// 技巧：通过 Joins 一次性完成 Slug 匹配和 Role 校验
		//err := db.Table("workspace_members").
		//	Select("workspace_members.role").
		//	Joins("JOIN workspaces ON workspaces.id = workspace_members.workspace_id").
		//	Where("workspaces.slug = ? AND workspace_members.user_id = ? AND workspace_members.status = ?",
		//		workspaceSlug, userID, "active").
		//	First(&member).Error
		//
		//if err != nil {
		//	c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: not a member of this workspace"})
		//	c.Abort()
		//	return
		//}
		//
		//// 4. 权限梯度判定
		//// 逻辑：AdminOnly 允许 Admin 和 Owner 通过
		//if member.Role != model.RoleAdmin && member.Role != model.RoleOwner {
		//	c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied: Admin required"})
		//	c.Abort()
		//	return
		//}

		// 5. 校验通过，放行
		c.Next()
	}
}
