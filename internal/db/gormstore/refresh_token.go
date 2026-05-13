package gormstore

import (
	"context"
	"time"

	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
)

type refreshTokenRepo struct {
	db *gorm.DB
}

func newRefreshTokenRepo(db *gorm.DB) *refreshTokenRepo {
	return &refreshTokenRepo{db: db}
}

func (r *refreshTokenRepo) Create(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *refreshTokenRepo) GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *refreshTokenRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("id = ?", id).
		Update("revoked_at", revokedAt).Error
}

func (r *refreshTokenRepo) RevokeAllByUser(ctx context.Context, userID string, revokedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", revokedAt).Error
}
