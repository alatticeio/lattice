package gormstore

import (
	"context"

	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type systemConfigRepo struct {
	db *gorm.DB
}

func newSystemConfigRepo(db *gorm.DB) *systemConfigRepo {
	return &systemConfigRepo{db: db}
}

func (r *systemConfigRepo) Get(ctx context.Context, key string) (string, error) {
	var cfg models.SystemConfig
	err := r.db.WithContext(ctx).Take(&cfg, "`key` = ?", key).Error
	if err != nil {
		return "", err
	}
	return cfg.Value, nil
}

func (r *systemConfigRepo) Set(ctx context.Context, key, value string) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&models.SystemConfig{Key: key, Value: value}).Error
}

func (r *systemConfigRepo) GetAll(ctx context.Context) (map[string]string, error) {
	var rows []models.SystemConfig
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		result[row.Key] = row.Value
	}
	return result, nil
}
