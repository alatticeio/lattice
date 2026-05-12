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

func (r *intentPlanRepo) MarkApplied(ctx context.Context, id, appliedBy string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.IntentPlan{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"applied":    true,
			"applied_at": &now,
			"applied_by": appliedBy,
		}).Error
}

func (r *intentPlanRepo) ListApplied(ctx context.Context, workspaceID string, limit int) ([]*models.IntentPlan, error) {
	var plans []*models.IntentPlan
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND applied = ?", workspaceID, true).
		Order("applied_at DESC").
		Limit(limit).
		Find(&plans).Error
	return plans, err
}

func (r *intentPlanRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ? AND applied = ?", time.Now(), false).
		Delete(&models.IntentPlan{}).Error
}

func (r *intentPlanRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.IntentPlan{}).Error
}
