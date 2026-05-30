package gormstore

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/repository"
	"gorm.io/gorm"
)

type agentIdentityRepo struct {
	*repository.BaseRepository[models.AgentIdentity]
}

func newAgentIdentityRepo(db *gorm.DB) *agentIdentityRepo {
	return &agentIdentityRepo{
		BaseRepository: repository.NewBaseRepository[models.AgentIdentity](db),
	}
}

func (r *agentIdentityRepo) ListByTenant(ctx context.Context, tenantID string) ([]*models.AgentIdentity, error) {
	return r.Find(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("tenant_id = ?", tenantID)
	})
}

func (r *agentIdentityRepo) GetByTenantAndName(ctx context.Context, tenantID, name string) (*models.AgentIdentity, error) {
	return r.First(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("tenant_id = ? AND name = ?", tenantID, name)
	})
}

func (r *agentIdentityRepo) GetByID(ctx context.Context, id string) (*models.AgentIdentity, error) {
	return r.BaseRepository.GetByID(ctx, id)
}

func (r *agentIdentityRepo) Delete(ctx context.Context, id string) error {
	return r.BaseRepository.Delete(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", id)
	})
}

var _ store.AgentIdentityRepository = (*agentIdentityRepo)(nil)
