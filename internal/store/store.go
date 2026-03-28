// Package store 定义数据库操作的纯接口层。
// 具体实现（SQLite / MariaDB）在 internal/db/gormstore 中，
// 通过 internal/db.NewStore 工厂函数按配置选择。
package store

import (
	"context"

	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/vo"
)

// Store 是顶层存储抽象，聚合所有子 Repository。
// 调用方只依赖本接口，不感知底层数据库类型。
type Store interface {
	Users() UserRepository
	Tokens() TokenRepository
	Workspaces() WorkspaceRepository
	WorkspaceMembers() WorkspaceMemberRepository
	Peers() PeerRepository
	Profiles() ProfileRepository

	// Tx 在同一个数据库事务中执行 fn，fn 内通过参数 s 访问所有 Repository。
	Tx(ctx context.Context, fn func(s Store) error) error

	Close() error
}

// UserRepository 定义用户相关数据操作。
type UserRepository interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByExternalID(ctx context.Context, externalID string) (*models.User, error)
	Login(ctx context.Context, username, password string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error)
	Count(ctx context.Context) (int64, error)
}

// TokenRepository 定义接入令牌相关数据操作。
type TokenRepository interface {
	GetByID(ctx context.Context, id string) (*models.Token, error)
	GetByToken(ctx context.Context, token string) (*models.Token, error)
	Create(ctx context.Context, token *models.Token) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, namespace string) ([]*models.Token, error)
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

// PeerRepository 定义节点相关数据操作。
type PeerRepository interface {
	GetByID(ctx context.Context, id string) (*models.Peer, error)
	GetByPublicKey(ctx context.Context, publicKey string) (*models.Peer, error)
	Create(ctx context.Context, peer *models.Peer) error
	Update(ctx context.Context, peer *models.Peer) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, appID string) ([]*models.Peer, error)
}

// ProfileRepository 定义用户扩展资料数据操作。
type ProfileRepository interface {
	Get(ctx context.Context, userID string) (*models.UserProfile, error)
	Upsert(ctx context.Context, profile *models.UserProfile) error
}
