package gormstore

import (
	"context"

	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/repository"

	"gorm.io/gorm"
)

// ── WorkspaceRepository ────────────────────────────────────────────────────

type workspaceRepo struct {
	*repository.BaseRepository[models.Workspace]
}

func newWorkspaceRepo(db *gorm.DB) *workspaceRepo {
	return &workspaceRepo{BaseRepository: repository.NewBaseRepository[models.Workspace](db)}
}

func (r *workspaceRepo) GetByID(ctx context.Context, id string) (*models.Workspace, error) {
	return r.BaseRepository.GetByID(ctx, id)
}

func (r *workspaceRepo) GetByNamespace(ctx context.Context, namespace string) (*models.Workspace, error) {
	return r.First(ctx, repository.WithNamespace(namespace))
}

func (r *workspaceRepo) Delete(ctx context.Context, id string) error {
	return r.BaseRepository.Delete(ctx, repository.WithID(id))
}

func (r *workspaceRepo) ListByUser(ctx context.Context, userID string) ([]*models.Workspace, error) {
	var workspaces []*models.Workspace
	err := r.DB().WithContext(ctx).
		Joins("JOIN t_workspaces_member ON t_workspaces_member.workspace_id = t_workspace.id").
		Where("t_workspaces_member.user_id = ? AND t_workspaces_member.deleted_at IS NULL", userID).
		Find(&workspaces).Error
	return workspaces, err
}

func (r *workspaceRepo) List(ctx context.Context, keyword string, page, pageSize int) ([]*models.Workspace, int64, error) {
	total, err := r.Count(ctx, repository.WithKeyword(keyword, "display_name", "slug"))
	if err != nil {
		return nil, 0, err
	}
	items, err := r.Find(ctx, repository.WithKeyword(keyword, "display_name", "slug"), repository.Paginate(page, pageSize))
	return items, total, err
}

// ── WorkspaceMemberRepository ──────────────────────────────────────────────

type workspaceMemberRepo struct {
	*repository.BaseRepository[models.WorkspaceMember]
}

func newWorkspaceMemberRepo(db *gorm.DB) *workspaceMemberRepo {
	return &workspaceMemberRepo{BaseRepository: repository.NewBaseRepository[models.WorkspaceMember](db)}
}

func (r *workspaceMemberRepo) GetMembership(ctx context.Context, workspaceID, userID string) (*models.WorkspaceMember, error) {
	return r.First(ctx, repository.WithWorkspaceID(workspaceID), repository.WithUserID(userID))
}

func (r *workspaceMemberRepo) AddMember(ctx context.Context, member *models.WorkspaceMember) error {
	return r.Create(ctx, member)
}

func (r *workspaceMemberRepo) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	return r.Delete(ctx, repository.WithWorkspaceID(workspaceID), repository.WithUserID(userID))
}

func (r *workspaceMemberRepo) DeleteByWorkspace(ctx context.Context, workspaceID string) error {
	return r.Delete(ctx, repository.WithWorkspaceID(workspaceID))
}

func (r *workspaceMemberRepo) ListMembers(ctx context.Context, workspaceID string) ([]*models.WorkspaceMember, error) {
	var members []*models.WorkspaceMember
	err := r.DB().WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Preload("User").
		Preload("User.Identities").
		Find(&members).Error
	return members, err
}

func (r *workspaceMemberRepo) ListByUser(ctx context.Context, userID string, page, pageSize int) ([]*models.WorkspaceMember, int64, error) {
	total, err := r.Count(ctx, repository.WithUserID(userID))
	if err != nil {
		return nil, 0, err
	}
	items, err := r.Find(ctx, repository.WithUserID(userID), repository.Paginate(page, pageSize))
	return items, total, err
}

func (r *workspaceMemberRepo) UpdateRole(ctx context.Context, workspaceID, userID string, role dto.WorkspaceRole) error {
	return r.DB().WithContext(ctx).
		Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Update("role", role).Error
}
