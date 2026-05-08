package repository

import (
	"context"
	"github.com/alatticeio/lattice/internal/agent/infra"

	"gorm.io/gorm"
)

// defind a scope, then repo can filter 'workspaceId' if exists.
func TenantScope(ctx context.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		wsID, _ := ctx.Value(infra.WorkspaceKey).(string)
		strict, _ := ctx.Value(infra.StrictTenantKey).(bool)

		// If there is no ID and it is not strict mode (e.g. Admin viewing all), do not filter
		if wsID == "" && !strict {
			return db
		}

		// As long as a wsID exists, whether for detail or list, force this filter condition
		return db.Where("workspace_id = ?", wsID)
	}
}
