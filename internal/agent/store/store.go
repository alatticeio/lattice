// Package store defines the pure interface layer for database operations.
// Concrete implementations (SQLite / MariaDB) live in internal/db/gormstore,
// selected by configuration via the internal/db.NewStore factory function.
package store

import (
	"context"
	"time"

	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/vo"
)

// Store is the top-level storage abstraction that aggregates all sub-Repository interfaces.
// Callers depend only on this interface without knowledge of the underlying database type.
// Peer and Token data have been migrated to K8s etcd (LatticePeer CRD / LatticeEnrollmentToken CRD).
type Store interface {
	Users() UserRepository
	Workspaces() WorkspaceRepository
	WorkspaceMembers() WorkspaceMemberRepository
	Profiles() ProfileRepository
	UserIdentities() UserIdentityRepository
	WorkspaceInvitations() WorkspaceInvitationRepository
	AuditLogs() AuditLogRepository
	WorkflowRequests() WorkflowRepository
	Policies() PolicyRepository

	// Tx executes fn within the same database transaction; fn accesses all Repository interfaces through parameter s.
	Tx(ctx context.Context, fn func(s Store) error) error

	Alerts() AlertRepository
	CustomMetrics() CustomMetricRepository
	SystemConfig() SystemConfigRepository
	IntentPlans() IntentPlanRepository
	NetworkSnapshots() NetworkSnapshotRepository
	AgentEnrollmentTokens() AgentEnrollmentTokenRepository
	Seed() SeedRepository
	RefreshTokens() RefreshTokenRepository
	ToolSpans() ToolSpanRepository
	FlowEvents() FlowEventRepository
	PeerIdentities() PeerIdentityRepository

	Close() error
}

// UserRepository defines user-related data operations.
type UserRepository interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Login(ctx context.Context, username, password string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error)
	// ListRaw returns raw User models with Identities preloaded, used for enriched VO mapping.
	ListRaw(ctx context.Context, req *dto.PageRequest) ([]*models.User, int64, error)
	Count(ctx context.Context) (int64, error)
}

// WorkspaceRepository defines workspace-related data operations.
type WorkspaceRepository interface {
	GetByID(ctx context.Context, id string) (*models.Workspace, error)
	GetByNamespace(ctx context.Context, namespace string) (*models.Workspace, error)
	Create(ctx context.Context, workspace *models.Workspace) error
	Update(ctx context.Context, workspace *models.Workspace) error
	Delete(ctx context.Context, id string) error
	ListByUser(ctx context.Context, userID string) ([]*models.Workspace, error)
	// List paginates workspaces by keyword, returning the result list and total count.
	List(ctx context.Context, keyword string, page, pageSize int) ([]*models.Workspace, int64, error)
	// ExistsByUserAndSlug checks whether the specified user already owns a workspace with the given slug.
	ExistsByUserAndSlug(ctx context.Context, userID, slug string) (bool, error)
	// ListExpiredDemos returns all demo workspaces whose ExpiresAt is before cutoff.
	ListExpiredDemos(ctx context.Context, cutoff time.Time) ([]*models.Workspace, error)
}

// WorkspaceMemberRepository defines workspace membership data operations.
type WorkspaceMemberRepository interface {
	GetMembership(ctx context.Context, workspaceID, userID string) (*models.WorkspaceMember, error)
	AddMember(ctx context.Context, member *models.WorkspaceMember) error
	RemoveMember(ctx context.Context, workspaceID, userID string) error
	SoftRemove(ctx context.Context, workspaceID, userID string) error
	DeleteByWorkspace(ctx context.Context, workspaceID string) error
	ListMembers(ctx context.Context, workspaceID string) ([]*models.WorkspaceMember, error)
	ListByUser(ctx context.Context, userID string, page, pageSize int) ([]*models.WorkspaceMember, int64, error)
	UpdateRole(ctx context.Context, workspaceID, userID string, role dto.WorkspaceRole) error
}

// ProfileRepository defines user profile data operations.
type ProfileRepository interface {
	Get(ctx context.Context, userID string) (*models.UserProfile, error)
	Upsert(ctx context.Context, profile *models.UserProfile) error
}

// UserIdentityRepository manages external identity provider links.
type UserIdentityRepository interface {
	GetByProviderAndExternalID(ctx context.Context, provider, externalID string) (*models.UserIdentity, error)
	ListByUser(ctx context.Context, userID string) ([]*models.UserIdentity, error)
	Create(ctx context.Context, identity *models.UserIdentity) error
}

// WorkspaceInvitationRepository manages workspace invitations.
type WorkspaceInvitationRepository interface {
	Create(ctx context.Context, inv *models.WorkspaceInvitation) error
	FindByID(ctx context.Context, id string) (*models.WorkspaceInvitation, error)
	GetByToken(ctx context.Context, token string) (*models.WorkspaceInvitation, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*models.WorkspaceInvitation, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	GetPendingByEmailAndWorkspace(ctx context.Context, email, workspaceID string) (*models.WorkspaceInvitation, error)
	// FindAcceptedByEmails returns the earliest accepted invitation for each of the given emails.
	FindAcceptedByEmails(ctx context.Context, emails []string) ([]*models.WorkspaceInvitation, error)
}

// AuditLogFilter defines query parameters for audit log listing.
type AuditLogFilter struct {
	WorkspaceID string
	Action      string
	Resource    string
	Status      string
	Keyword     string // searches UserName and ResourceName
	From        string // RFC3339 or date string
	To          string
	Page        int
	PageSize    int
}

// AuditLogRepository manages append-only audit log records.
type AuditLogRepository interface {
	Create(ctx context.Context, log *models.AuditLog) error
	BatchCreate(ctx context.Context, logs []*models.AuditLog) error
	List(ctx context.Context, filter AuditLogFilter) ([]*models.AuditLog, int64, error)
}

// WorkflowFilter defines query parameters for workflow request listing.
type WorkflowFilter struct {
	WorkspaceID  string
	RequestedBy  string
	ResourceType string
	Action       string
	Status       string
	Page         int
	PageSize     int
}

// PolicyFilter defines query parameters for policy listing.
type PolicyFilter struct {
	WorkspaceID string
	Status      string
	Keyword     string
	Page        int
	PageSize    int
}

// PolicyRepository manages policy DB records.
type PolicyRepository interface {
	Create(ctx context.Context, policy *models.Policy) error
	GetByID(ctx context.Context, id string) (*models.Policy, error)
	GetByName(ctx context.Context, workspaceID, name string) (*models.Policy, error)
	List(ctx context.Context, filter PolicyFilter) ([]*models.Policy, int64, error)
	Update(ctx context.Context, policy *models.Policy) error
	Delete(ctx context.Context, workspaceID, name string) error
}

// WorkflowRepository manages workflow approval requests.
type WorkflowRepository interface {
	Create(ctx context.Context, req *models.WorkflowRequest) error
	GetByID(ctx context.Context, id string) (*models.WorkflowRequest, error)
	UpdateStatus(ctx context.Context, id string, status models.WorkflowStatus, fields map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter WorkflowFilter) ([]*models.WorkflowRequest, int64, error)
}

// AlertRepository manages alert rules, history, channels, and silences.
type AlertRepository interface {
	GetAlertRule(ctx context.Context, id string) (*models.AlertRule, error)
	CreateAlertRule(ctx context.Context, rule *models.AlertRule) error
	UpdateAlertRule(ctx context.Context, rule *models.AlertRule) error
	DeleteAlertRule(ctx context.Context, id string) error
	ListAlertRulesByWorkspace(ctx context.Context, wsID string) ([]*models.AlertRule, error)
	ListEnabledAlertRules(ctx context.Context) ([]*models.AlertRule, error)

	ListAlertHistory(ctx context.Context, wsID string, page, pageSize int) ([]*models.AlertHistory, int64, error)
	CreateAlertHistory(ctx context.Context, h *models.AlertHistory) error
	UpdateAlertHistory(ctx context.Context, h *models.AlertHistory) error

	ListAlertChannels(ctx context.Context, wsID string) ([]*models.AlertChannel, error)
	CreateAlertChannel(ctx context.Context, c *models.AlertChannel) error
	UpdateAlertChannel(ctx context.Context, c *models.AlertChannel) error
	DeleteAlertChannel(ctx context.Context, id string) error
	GetAlertChannel(ctx context.Context, id string) (*models.AlertChannel, error)

	ListAlertSilences(ctx context.Context, wsID string) ([]*models.AlertSilence, error)
	CreateAlertSilence(ctx context.Context, s *models.AlertSilence) error
	DeleteAlertSilence(ctx context.Context, id string) error
}

// SystemConfigRepository manages platform-level key-value configuration.
type SystemConfigRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) (map[string]string, error)
}

// IntentPlanRepository manages intent plan records.
type IntentPlanRepository interface {
	Create(ctx context.Context, plan *models.IntentPlan) error
	GetByID(ctx context.Context, id string) (*models.IntentPlan, error)
	MarkApplied(ctx context.Context, id, appliedBy string) error
	ListApplied(ctx context.Context, workspaceID string, limit int) ([]*models.IntentPlan, error)
	DeleteExpired(ctx context.Context) error
	Delete(ctx context.Context, id string) error
}

// NetworkSnapshotRepository manages point-in-time network state captures.
type NetworkSnapshotRepository interface {
	Create(ctx context.Context, snap *models.NetworkSnapshot) error
	GetByID(ctx context.Context, id string) (*models.NetworkSnapshot, error)
	List(ctx context.Context, workspaceID string, from, to time.Time, triggerType string) ([]*models.NetworkSnapshot, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// CustomMetricRepository manages user-defined metric DB operations.
type CustomMetricRepository interface {
	GetByID(ctx context.Context, id string) (*models.CustomMetric, error)
	Create(ctx context.Context, m *models.CustomMetric) error
	Update(ctx context.Context, m *models.CustomMetric) error
	Delete(ctx context.Context, id string) error
	ListByWorkspace(ctx context.Context, wsID string) ([]*models.CustomMetric, error)
}

// AgentEnrollmentTokenRepository manages one-time registration tokens.
type AgentEnrollmentTokenRepository interface {
	Create(ctx context.Context, token *models.AgentEnrollmentToken) error
	GetByToken(ctx context.Context, token string) (*models.AgentEnrollmentToken, error)
	MarkUsed(ctx context.Context, token string, usedAt time.Time) error
	DeleteExpired(ctx context.Context) error
}

// RefreshTokenRepository manages refresh token lifecycle.
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *models.RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, id string, revokedAt time.Time) error
	RevokeAllByUser(ctx context.Context, userID string, revokedAt time.Time) error
}

// SeedRepository handles demo seed data lifecycle.
type SeedRepository interface {
	Clear(ctx context.Context, workspaceID string) error
}

// ToolSpanRepository records and queries MCP tool call spans.
type ToolSpanRepository interface {
	Write(ctx context.Context, span *models.ToolSpan) error
	Get(ctx context.Context, traceID string) (*models.ToolSpan, error)
	List(ctx context.Context, agentID string, from, to time.Time, limit int) ([]*models.ToolSpan, error)
}

// FlowEventRepository records and queries gVisor network flow events.
type FlowEventRepository interface {
	Write(ctx context.Context, e *models.FlowEvent) error
	ListByTrace(ctx context.Context, traceID string) ([]*models.FlowEvent, error)
}

// PeerIdentityRepository manages stable logical identities for devices.
type PeerIdentityRepository interface {
	GetByID(ctx context.Context, id string) (*models.PeerIdentity, error)
	GetByNetworkAndName(ctx context.Context, networkID, name string) (*models.PeerIdentity, error)
	Create(ctx context.Context, m *models.PeerIdentity) error
	Update(ctx context.Context, m *models.PeerIdentity) error
	Delete(ctx context.Context, id string) error
	ListByNetwork(ctx context.Context, networkID string) ([]*models.PeerIdentity, error)
}
