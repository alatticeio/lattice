package gormstore

import (
	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
)

// migrate automatically synchronizes all table structures on startup.
// GORM AutoMigrate only performs incremental changes (new columns/indexes), never drops columns,
// and is safe for existing data.
// Token and Peer data has been migrated to K8s etcd and is no longer managed here.
func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.UserProfile{},
		&models.UserIdentity{},
		&models.Workspace{},
		&models.WorkspaceMember{},
		&models.WorkspaceInvitation{},
		&models.AuditLog{},
		&models.WorkflowRequest{},
		&models.Policy{},
		&models.AlertRule{},
		&models.AlertHistory{},
		&models.AlertChannel{},
		&models.AlertSilence{},
		&models.CustomMetric{},
		&models.SystemConfig{},
		&models.IntentPlan{},
		&models.NetworkSnapshot{},
	)
}
