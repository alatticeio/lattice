package gormstore

import (
	"context"

	"wireflow/management/models"
	"wireflow/management/repository"

	"gorm.io/gorm"
)

type profileRepo struct {
	*repository.BaseRepository[models.UserProfile]
}

func newProfileRepo(db *gorm.DB) *profileRepo {
	return &profileRepo{BaseRepository: repository.NewBaseRepository[models.UserProfile](db)}
}

func (r *profileRepo) Get(ctx context.Context, userID string) (*models.UserProfile, error) {
	return r.First(ctx, repository.WithUserID(userID))
}

func (r *profileRepo) Upsert(ctx context.Context, profile *models.UserProfile) error {
	return r.BaseRepository.Upsert(ctx,
		models.UserProfile{UserID: profile.UserID},
		*profile,
	)
}
