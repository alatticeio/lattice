package repository

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/management/utils"
	"wireflow/pkg/log"

	"gorm.io/gorm"
)

type PolicyRepository interface {
	WithTx(tx *gorm.DB) PolicyRepository
	Create(ctx context.Context, accessPolicy *entity.AccessPolicy) error
	Delete(ctx context.Context, accessId uint64) error
	Update(ctx context.Context, accessPolicy *entity.AccessPolicy) error
	Find(ctx context.Context, accessId uint64) (*entity.AccessPolicy, error)
	List(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.AccessPolicy, int64, error)
	Query(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.AccessPolicy, error)
}

var (
	_ PolicyRepository = (*policyRepository)(nil)
)

type policyRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewPolicyRepository(db *gorm.DB) PolicyRepository {
	return &policyRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "access-policy-repository"),
	}
}

func (r *policyRepository) WithTx(tx *gorm.DB) PolicyRepository {
	return NewPolicyRepository(tx)
}

func (r *policyRepository) Create(ctx context.Context, access *entity.AccessPolicy) error {
	return r.db.WithContext(ctx).Create(access).Error
}

func (r *policyRepository) Delete(ctx context.Context, accessId uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.AccessPolicy{}, accessId).Error
}

func (r *policyRepository) Update(ctx context.Context, accessPolicy *entity.AccessPolicy) error {
	return r.db.WithContext(ctx).Model(&entity.AccessPolicy{}).Where("id=?", accessPolicy.ID).Updates(map[string]interface{}{
		"name":        accessPolicy.Name,
		"effect":      accessPolicy.Effect,
		"status":      accessPolicy.Status,
		"description": accessPolicy.Description,
		"priority":    accessPolicy.Priority,
	}).Error
}

func (r *policyRepository) Find(ctx context.Context, accessId uint64) (*entity.AccessPolicy, error) {
	var access entity.AccessPolicy
	err := r.db.WithContext(ctx).First(&access, accessId).Error
	if err != nil {
		return nil, err
	}
	return &access, nil
}

func (r *policyRepository) List(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.AccessPolicy, int64, error) {
	var (
		policies []*entity.AccessPolicy
		count    int64
		err      error
	)

	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.AccessPolicy{}))

	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	//5. query
	if err := query.Find(&policies).Error; err != nil {
		return nil, 0, err
	}

	return policies, count, nil
}

func (r *policyRepository) Query(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.AccessPolicy, error) {
	var policies []*entity.AccessPolicy
	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.AccessPolicy{}))
	if err := query.Find(&policies).Error; err != nil {
		return nil, err
	}

	return policies, nil
}
