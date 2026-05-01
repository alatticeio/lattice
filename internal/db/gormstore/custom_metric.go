package gormstore

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/repository"
	"gorm.io/gorm"
)

type customMetricRepo struct {
	*repository.BaseRepository[models.CustomMetric]
}

func newCustomMetricRepo(db *gorm.DB) *customMetricRepo {
	return &customMetricRepo{
		BaseRepository: repository.NewBaseRepository[models.CustomMetric](db),
	}
}

func (r *customMetricRepo) ListByWorkspace(ctx context.Context, wsID string) ([]*models.CustomMetric, error) {
	return r.BaseRepository.Find(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("workspace_id = ?", wsID)
	})
}

func (r *customMetricRepo) GetByID(ctx context.Context, id string) (*models.CustomMetric, error) {
	return r.BaseRepository.GetByID(ctx, id)
}

func (r *customMetricRepo) Delete(ctx context.Context, id string) error {
	return r.BaseRepository.Delete(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", id)
	})
}

var _ store.CustomMetricRepository = (*customMetricRepo)(nil)
