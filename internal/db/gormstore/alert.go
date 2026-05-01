package gormstore

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/repository"
	"gorm.io/gorm"
)

type alertRepo struct {
	ruleRepo    *repository.BaseRepository[models.AlertRule]
	historyRepo *repository.BaseRepository[models.AlertHistory]
	channelRepo *repository.BaseRepository[models.AlertChannel]
	silenceRepo *repository.BaseRepository[models.AlertSilence]
}

func newAlertRepo(db *gorm.DB) *alertRepo {
	return &alertRepo{
		ruleRepo:    repository.NewBaseRepository[models.AlertRule](db),
		historyRepo: repository.NewBaseRepository[models.AlertHistory](db),
		channelRepo: repository.NewBaseRepository[models.AlertChannel](db),
		silenceRepo: repository.NewBaseRepository[models.AlertSilence](db),
	}
}

func (r *alertRepo) GetAlertRule(ctx context.Context, id string) (*models.AlertRule, error) {
	return r.ruleRepo.GetByID(ctx, id)
}

func (r *alertRepo) CreateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	return r.ruleRepo.Create(ctx, rule)
}

func (r *alertRepo) UpdateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	return r.ruleRepo.Update(ctx, rule)
}

func (r *alertRepo) DeleteAlertRule(ctx context.Context, id string) error {
	return r.ruleRepo.Delete(ctx, repository.WithID(id))
}

func (r *alertRepo) ListAlertRulesByWorkspace(ctx context.Context, wsID string) ([]*models.AlertRule, error) {
	return r.ruleRepo.Find(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("workspace_id = ?", wsID)
	})
}

func (r *alertRepo) ListEnabledAlertRules(ctx context.Context) ([]*models.AlertRule, error) {
	return r.ruleRepo.Find(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("enabled = ?", true)
	})
}

func (r *alertRepo) ListAlertHistory(ctx context.Context, wsID string, page, pageSize int) ([]*models.AlertHistory, int64, error) {
	var total int64
	q := r.historyRepo.DB().WithContext(ctx).Where("workspace_id = ?", wsID)
	if err := q.Model(&models.AlertHistory{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*models.AlertHistory
	err := q.Order("started_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func (r *alertRepo) CreateAlertHistory(ctx context.Context, h *models.AlertHistory) error {
	return r.historyRepo.Create(ctx, h)
}

func (r *alertRepo) UpdateAlertHistory(ctx context.Context, h *models.AlertHistory) error {
	return r.historyRepo.Update(ctx, h)
}

func (r *alertRepo) ListAlertChannels(ctx context.Context, wsID string) ([]*models.AlertChannel, error) {
	return r.channelRepo.Find(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("workspace_id = ?", wsID)
	})
}

func (r *alertRepo) CreateAlertChannel(ctx context.Context, c *models.AlertChannel) error {
	return r.channelRepo.Create(ctx, c)
}

func (r *alertRepo) UpdateAlertChannel(ctx context.Context, c *models.AlertChannel) error {
	return r.channelRepo.Update(ctx, c)
}

func (r *alertRepo) DeleteAlertChannel(ctx context.Context, id string) error {
	return r.channelRepo.Delete(ctx, repository.WithID(id))
}

func (r *alertRepo) GetAlertChannel(ctx context.Context, id string) (*models.AlertChannel, error) {
	return r.channelRepo.GetByID(ctx, id)
}

func (r *alertRepo) ListAlertSilences(ctx context.Context, wsID string) ([]*models.AlertSilence, error) {
	return r.silenceRepo.Find(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("workspace_id = ?", wsID)
	})
}

func (r *alertRepo) CreateAlertSilence(ctx context.Context, s *models.AlertSilence) error {
	return r.silenceRepo.Create(ctx, s)
}

func (r *alertRepo) DeleteAlertSilence(ctx context.Context, id string) error {
	return r.silenceRepo.Delete(ctx, repository.WithID(id))
}

var _ store.AlertRepository = (*alertRepo)(nil)
