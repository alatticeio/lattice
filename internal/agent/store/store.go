// Package store 定义数据库操作的纯接口层。
// 具体实现（SQLite / MariaDB）在 internal/db/gormstore 中，
// 通过 internal/db.NewStore 工厂函数按配置选择。
package store

import (
	"context"

	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/vo"
)

// Store 是顶层存储抽象，聚合所有子 Repository。
// 调用方只依赖本接口，不感知底层数据库类型。
// Peer 和 Token 数据已迁移到 K8s etcd（LatticePeer CRD / LatticeEnrollmentToken CRD）。
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

	// Tx 在同一个数据库事务中执行 fn，fn 内通过参数 s 访问所有 Repository。
	Tx(ctx context.Context, fn func(s Store) error) error

	Close() error
}

// UserRepository 定义用户相关数据操作。
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

// WorkspaceRepository 定义工作空间相关数据操作。
type WorkspaceRepository interface {
	GetByID(ctx context.Context, id string) (*models.Workspace, error)
	GetByNamespace(ctx context.Context, namespace string) (*models.Workspace, error)
	Create(ctx context.Context, workspace *models.Workspace) error
	Update(ctx context.Context, workspace *models.Workspace) error
	Delete(ctx context.Context, id string) error
	ListByUser(ctx context.Context, userID string) ([]*models.Workspace, error)
	// List 按关键字分页列举工作空间，返回结果列表和总数。
	List(ctx context.Context, keyword string, page, pageSize int) ([]*models.Workspace, int64, error)
	// ExistsByUserAndSlug 检查指定用户是否已拥有同名（slug）工作空间。
	ExistsByUserAndSlug(ctx context.Context, userID, slug string) (bool, error)
}

// WorkspaceMemberRepository 定义工作空间成员关系数据操作。
type WorkspaceMemberRepository interface {
	GetMembership(ctx context.Context, workspaceID, userID string) (*models.WorkspaceMember, error)
	AddMember(ctx context.Context, member *models.WorkspaceMember) error
	RemoveMember(ctx context.Context, workspaceID, userID string) error
	DeleteByWorkspace(ctx context.Context, workspaceID string) error
	ListMembers(ctx context.Context, workspaceID string) ([]*models.WorkspaceMember, error)
	ListByUser(ctx context.Context, userID string, page, pageSize int) ([]*models.WorkspaceMember, int64, error)
	UpdateRole(ctx context.Context, workspaceID, userID string, role dto.WorkspaceRole) error
}

// ProfileRepository 定义用户扩展资料数据操作。
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
