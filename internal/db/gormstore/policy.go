package gormstore

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
)

type policyRepo struct {
	db *gorm.DB
}

func newPolicyRepo(db *gorm.DB) *policyRepo {
	return &policyRepo{db: db}
}

func (r *policyRepo) Create(ctx context.Context, policy *models.Policy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

func (r *policyRepo) GetByID(ctx context.Context, id string) (*models.Policy, error) {
	var p models.Policy
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *policyRepo) GetByName(ctx context.Context, workspaceID, name string) (*models.Policy, error) {
	var p models.Policy
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND name = ?", workspaceID, name).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *policyRepo) List(ctx context.Context, filter store.PolicyFilter) ([]*models.Policy, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Policy{})

	if filter.WorkspaceID != "" {
		q = q.Where("workspace_id = ?", filter.WorkspaceID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		q = q.Where("name LIKE ? OR description LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var policies []*models.Policy
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&policies).Error
	return policies, total, err
}

func (r *policyRepo) Update(ctx context.Context, policy *models.Policy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

func (r *policyRepo) Delete(ctx context.Context, workspaceID, name string) error {
	return r.db.WithContext(ctx).
		Where("workspace_id = ? AND name = ?", workspaceID, name).
		Delete(&models.Policy{}).Error
}
