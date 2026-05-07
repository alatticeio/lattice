package gormstore

import (
	"context"
	"time"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
)

type intentPlanRepo struct{ db *gorm.DB }

func newIntentPlanRepo(db *gorm.DB) store.IntentPlanRepository {
	return &intentPlanRepo{db: db}
}

func (r *intentPlanRepo) Create(ctx context.Context, plan *models.IntentPlan) error {
	return r.db.WithContext(ctx).Create(plan).Error
}

func (r *intentPlanRepo) GetByID(ctx context.Context, id string) (*models.IntentPlan, error) {
	var plan models.IntentPlan
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&plan).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *intentPlanRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&models.IntentPlan{}).Error
}

func (r *intentPlanRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.IntentPlan{}).Error
}
