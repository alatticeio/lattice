package repository

import (
	"context"

	"gorm.io/gorm"
)

// defind a scope, then repo can filter 'workspaceId' if exists.
func TenantScope(ctx context.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		wsID, _ := ctx.Value("workspaceId").(string)
		strict, _ := ctx.Value("isStrictTenant").(bool)

		// 如果没有 ID 且不是严格模式（如 Admin 看全量），则不加过滤
		if wsID == "" && !strict {
			return db
		}

		// 只要有 wsID，无论是详情还是列表，都强制带上这个过滤条件
		return db.Where("workspace_id = ?", wsID)
	}
}
