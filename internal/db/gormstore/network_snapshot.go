package gormstore

import (
	"context"
	"time"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
)

type networkSnapshotRepo struct{ db *gorm.DB }

func newNetworkSnapshotRepo(db *gorm.DB) store.NetworkSnapshotRepository {
	return &networkSnapshotRepo{db: db}
}

func (r *networkSnapshotRepo) Create(ctx context.Context, snap *models.NetworkSnapshot) error {
	return r.db.WithContext(ctx).Create(snap).Error
}

func (r *networkSnapshotRepo) GetByID(ctx context.Context, id string) (*models.NetworkSnapshot, error) {
	var snap models.NetworkSnapshot
	if err := r.db.WithContext(ctx).First(&snap, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

func (r *networkSnapshotRepo) List(ctx context.Context, workspaceID string, from, to time.Time, triggerType string) ([]*models.NetworkSnapshot, error) {
	q := r.db.WithContext(ctx).
		Where("workspace_id = ? AND captured_at BETWEEN ? AND ?", workspaceID, from, to).
		Order("captured_at DESC")
	if triggerType != "" {
		q = q.Where("trigger_type = ?", triggerType)
	}
	var snaps []*models.NetworkSnapshot
	return snaps, q.Find(&snaps).Error
}

func (r *networkSnapshotRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("captured_at < ?", before).
		Delete(&models.NetworkSnapshot{})
	return result.RowsAffected, result.Error
}
