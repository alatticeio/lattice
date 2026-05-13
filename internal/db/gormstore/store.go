// Package gormstore provides a unified Store implementation based on GORM,
// supporting both SQLite (open-source default) and MySQL/MariaDB (production).
// Both use the same CRUD logic, differing only in the GORM dialect,
// which is selected in the upper-level factory internal/db.NewStore.
package gormstore

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"

	"gorm.io/gorm"
)

// GormStore implements the store.Store interface.
// Peer and Token have been migrated to K8s etcd and are no longer managed by this store.
type GormStore struct {
	db                    *gorm.DB
	users                 store.UserRepository
	workspaces            store.WorkspaceRepository
	workspaceMembers      store.WorkspaceMemberRepository
	profiles              store.ProfileRepository
	userIdentities        store.UserIdentityRepository
	workspaceInvitations  store.WorkspaceInvitationRepository
	auditLogs             store.AuditLogRepository
	workflowRequests      store.WorkflowRepository
	policies              store.PolicyRepository
	alerts                store.AlertRepository
	customMetrics         store.CustomMetricRepository
	systemConfig          store.SystemConfigRepository
	intentPlans           store.IntentPlanRepository
	networkSnapshots      store.NetworkSnapshotRepository
	agentEnrollmentTokens store.AgentEnrollmentTokenRepository
	refreshTokens         store.RefreshTokenRepository
	seed                  store.SeedRepository
}

// New creates the gormStore: first runs AutoMigrate, then initializes each sub-Repository.
func New(db *gorm.DB) (store.Store, error) {
	if err := migrate(db); err != nil {
		return nil, err
	}
	return newStore(db), nil
}
func newStore(db *gorm.DB) *GormStore {
	return &GormStore{
		db:                    db,
		users:                 newUserRepo(db),
		workspaces:            newWorkspaceRepo(db),
		workspaceMembers:      newWorkspaceMemberRepo(db),
		profiles:              newProfileRepo(db),
		userIdentities:        newUserIdentityRepo(db),
		workspaceInvitations:  newWorkspaceInvitationRepo(db),
		auditLogs:             newAuditLogRepo(db),
		workflowRequests:      newWorkflowRepo(db),
		policies:              newPolicyRepo(db),
		alerts:                newAlertRepo(db),
		customMetrics:         newCustomMetricRepo(db),
		systemConfig:          newSystemConfigRepo(db),
		intentPlans:           newIntentPlanRepo(db),
		networkSnapshots:      newNetworkSnapshotRepo(db),
		seed:                  newSeedRepo(db),
		agentEnrollmentTokens: NewAgentEnrollmentTokenRepo(db),
		refreshTokens:         newRefreshTokenRepo(db),
	}
}

func (s *GormStore) Users() store.UserRepository                       { return s.users }
func (s *GormStore) Workspaces() store.WorkspaceRepository             { return s.workspaces }
func (s *GormStore) WorkspaceMembers() store.WorkspaceMemberRepository { return s.workspaceMembers }
func (s *GormStore) Profiles() store.ProfileRepository                 { return s.profiles }
func (s *GormStore) UserIdentities() store.UserIdentityRepository      { return s.userIdentities }
func (s *GormStore) WorkspaceInvitations() store.WorkspaceInvitationRepository {
	return s.workspaceInvitations
}
func (s *GormStore) AuditLogs() store.AuditLogRepository               { return s.auditLogs }
func (s *GormStore) WorkflowRequests() store.WorkflowRepository        { return s.workflowRequests }
func (s *GormStore) Policies() store.PolicyRepository                  { return s.policies }
func (s *GormStore) Alerts() store.AlertRepository                     { return s.alerts }
func (s *GormStore) CustomMetrics() store.CustomMetricRepository       { return s.customMetrics }
func (s *GormStore) SystemConfig() store.SystemConfigRepository        { return s.systemConfig }
func (s *GormStore) IntentPlans() store.IntentPlanRepository           { return s.intentPlans }
func (s *GormStore) NetworkSnapshots() store.NetworkSnapshotRepository { return s.networkSnapshots }
func (s *GormStore) AgentEnrollmentTokens() store.AgentEnrollmentTokenRepository {
	return s.agentEnrollmentTokens
}
func (s *GormStore) Seed() store.SeedRepository { return s.seed }
func (s *GormStore) RefreshTokens() store.RefreshTokenRepository { return s.refreshTokens }

// Tx executes fn within a database transaction, providing a temporary Store for all Repository access.
func (s *GormStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(newStore(tx))
	})
}

func (s *GormStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// DB returns the underlying *gorm.DB for components that need direct access.
func (s *GormStore) DB() *gorm.DB { return s.db }
