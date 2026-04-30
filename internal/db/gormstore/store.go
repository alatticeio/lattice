// Package gormstore 提供基于 GORM 的 Store 统一实现，
// 同时支持 SQLite（开源默认）和 MySQL/MariaDB（生产环境）。
// 两者使用同一套 CRUD 逻辑，仅 GORM dialect 不同，
// dialect 的选择在上层工厂 internal/db.NewStore 中完成。
package gormstore

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"

	"gorm.io/gorm"
)

// gormStore 实现 store.Store 接口。
// Peer 和 Token 已迁移至 K8s etcd，不再由此 store 管理。
type gormStore struct {
	db                   *gorm.DB
	users                store.UserRepository
	workspaces           store.WorkspaceRepository
	workspaceMembers     store.WorkspaceMemberRepository
	profiles             store.ProfileRepository
	userIdentities       store.UserIdentityRepository
	workspaceInvitations store.WorkspaceInvitationRepository
	auditLogs            store.AuditLogRepository
	workflowRequests     store.WorkflowRepository
	policies             store.PolicyRepository
}

// New 创建 gormStore：先执行 AutoMigrate，再初始化各子 Repository。
func New(db *gorm.DB) (store.Store, error) {
	if err := migrate(db); err != nil {
		return nil, err
	}
	return newStore(db), nil
}

func newStore(db *gorm.DB) *gormStore {
	return &gormStore{
		db:                   db,
		users:                newUserRepo(db),
		workspaces:           newWorkspaceRepo(db),
		workspaceMembers:     newWorkspaceMemberRepo(db),
		profiles:             newProfileRepo(db),
		userIdentities:       newUserIdentityRepo(db),
		workspaceInvitations: newWorkspaceInvitationRepo(db),
		auditLogs:            newAuditLogRepo(db),
		workflowRequests:     newWorkflowRepo(db),
		policies:             newPolicyRepo(db),
	}
}

func (s *gormStore) Users() store.UserRepository                       { return s.users }
func (s *gormStore) Workspaces() store.WorkspaceRepository             { return s.workspaces }
func (s *gormStore) WorkspaceMembers() store.WorkspaceMemberRepository { return s.workspaceMembers }
func (s *gormStore) Profiles() store.ProfileRepository                 { return s.profiles }
func (s *gormStore) UserIdentities() store.UserIdentityRepository      { return s.userIdentities }
func (s *gormStore) WorkspaceInvitations() store.WorkspaceInvitationRepository {
	return s.workspaceInvitations
}
func (s *gormStore) AuditLogs() store.AuditLogRepository        { return s.auditLogs }
func (s *gormStore) WorkflowRequests() store.WorkflowRepository { return s.workflowRequests }
func (s *gormStore) Policies() store.PolicyRepository           { return s.policies }

// Tx 在数据库事务中执行 fn，fn 内通过临时 Store 访问所有 Repository。
func (s *gormStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(newStore(tx))
	})
}

func (s *gormStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
