package gormstore

import (
	"context"
	"wireflow/management/models"
	"wireflow/management/repository"

	"gorm.io/gorm"
)

type workspaceInvitationRepo struct {
	*repository.BaseRepository[models.WorkspaceInvitation]
}

func newWorkspaceInvitationRepo(db *gorm.DB) *workspaceInvitationRepo {
	return &workspaceInvitationRepo{BaseRepository: repository.NewBaseRepository[models.WorkspaceInvitation](db)}
}

func (r *workspaceInvitationRepo) Create(ctx context.Context, inv *models.WorkspaceInvitation) error {
	return r.BaseRepository.Create(ctx, inv)
}

func (r *workspaceInvitationRepo) FindByID(ctx context.Context, id string) (*models.WorkspaceInvitation, error) {
	return r.First(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", id)
	})
}

func (r *workspaceInvitationRepo) GetByToken(ctx context.Context, token string) (*models.WorkspaceInvitation, error) {
	return r.First(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("token = ?", token)
	})
}

func (r *workspaceInvitationRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]*models.WorkspaceInvitation, error) {
	return r.Find(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("workspace_id = ?", workspaceID).Order("created_at DESC")
	})
}

func (r *workspaceInvitationRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	return r.DB().WithContext(ctx).
		Model(&models.WorkspaceInvitation{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *workspaceInvitationRepo) FindAcceptedByEmails(ctx context.Context, emails []string) ([]*models.WorkspaceInvitation, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	return r.Find(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("email IN ? AND status = 'accepted'", emails).Order("created_at ASC")
	})
}

func (r *workspaceInvitationRepo) GetPendingByEmailAndWorkspace(ctx context.Context, email, workspaceID string) (*models.WorkspaceInvitation, error) {
	return r.First(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("email = ? AND workspace_id = ? AND status = 'pending'", email, workspaceID)
	})
}
