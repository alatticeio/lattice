package gormstore

import (
	"context"

	"wireflow/management/models"
	"wireflow/management/repository"

	"gorm.io/gorm"
)

type tokenRepo struct {
	*repository.BaseRepository[models.Token]
}

func newTokenRepo(db *gorm.DB) *tokenRepo {
	return &tokenRepo{BaseRepository: repository.NewBaseRepository[models.Token](db)}
}

func (r *tokenRepo) GetByID(ctx context.Context, id string) (*models.Token, error) {
	return r.BaseRepository.GetByID(ctx, id)
}

func (r *tokenRepo) GetByToken(ctx context.Context, tokenStr string) (*models.Token, error) {
	return r.First(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("token = ?", tokenStr)
	})
}

func (r *tokenRepo) Delete(ctx context.Context, id string) error {
	return r.BaseRepository.Delete(ctx, repository.WithID(id))
}

func (r *tokenRepo) List(ctx context.Context, namespace string) ([]*models.Token, error) {
	return r.Find(ctx, func(db *gorm.DB) *gorm.DB {
		q := db.Order("created_at DESC")
		if namespace != "" {
			q = q.Where("namespace = ?", namespace)
		}
		return q
	})
}
