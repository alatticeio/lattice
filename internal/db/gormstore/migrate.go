package gormstore

import (
	"wireflow/management/models"

	"gorm.io/gorm"
)

// migrate 在启动时自动同步所有表结构。
// GORM AutoMigrate 仅做增量变更（新增列/索引），不会删除列，对存量数据安全。
// Token 和 Peer 数据已迁移至 K8s etcd，不再在此处管理。
func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.UserProfile{},
		&models.Workspace{},
		&models.WorkspaceMember{},
	)
}
