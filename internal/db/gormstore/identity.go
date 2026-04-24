package gormstore

import (
	"context"
	"wireflow/management/models"
	"wireflow/management/repository"

	"gorm.io/gorm"
)

type userIdentityRepo struct {
	*repository.BaseRepository[models.UserIdentity]
}

func newUserIdentityRepo(db *gorm.DB) *userIdentityRepo {
	return &userIdentityRepo{BaseRepository: repository.NewBaseRepository[models.UserIdentity](db)}
}

func (r *userIdentityRepo) GetByProviderAndExternalID(ctx context.Context, provider, externalID string) (*models.UserIdentity, error) {
	return r.First(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("provider = ? AND external_id = ?", provider, externalID)
	})
}

func (r *userIdentityRepo) ListByUser(ctx context.Context, userID string) ([]*models.UserIdentity, error) {
	return r.Find(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("user_id = ?", userID)
	})
}

func (r *userIdentityRepo) Create(ctx context.Context, identity *models.UserIdentity) error {
	return r.BaseRepository.Create(ctx, identity)
}
