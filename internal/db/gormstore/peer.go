package gormstore

import (
	"context"

	"wireflow/management/models"
	"wireflow/management/repository"

	"gorm.io/gorm"
)

type peerRepo struct {
	*repository.BaseRepository[models.Peer]
}

func newPeerRepo(db *gorm.DB) *peerRepo {
	return &peerRepo{BaseRepository: repository.NewBaseRepository[models.Peer](db)}
}

func (r *peerRepo) GetByID(ctx context.Context, id string) (*models.Peer, error) {
	return r.BaseRepository.GetByID(ctx, id)
}

func (r *peerRepo) GetByPublicKey(ctx context.Context, publicKey string) (*models.Peer, error) {
	return r.First(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("public_key = ?", publicKey)
	})
}

func (r *peerRepo) Delete(ctx context.Context, id string) error {
	return r.BaseRepository.Delete(ctx, repository.WithID(id))
}

func (r *peerRepo) List(ctx context.Context, appID string) ([]*models.Peer, error) {
	return r.Find(ctx, func(db *gorm.DB) *gorm.DB {
		q := db.Order("created_at DESC")
		if appID != "" {
			q = q.Where("app_id = ?", appID)
		}
		return q
	})
}
