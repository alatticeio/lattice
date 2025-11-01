package repository

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/pkg/log"
	"wireflow/pkg/utils"

	"gorm.io/gorm"
)

type GroupPolicyRepository interface {
	WithTx(tx *gorm.DB) GroupPolicyRepository
	Create(ctx context.Context, groupPolicy *entity.GroupPolicy) error
	Delete(ctx context.Context, id uint64) error
	DeleteByGroupPolicyId(ctx context.Context, groupId, policyId uint64) error
	Update(ctx context.Context, dto *dto.GroupPolicyDto) error
	Find(ctx context.Context, id uint64) (*entity.GroupPolicy, error)
	FindByGroupNodeId(ctx context.Context, groupId, nodeId uint64) (*entity.GroupPolicy, error)

	List(ctx context.Context, params *dto.GroupPolicyParams) ([]*entity.GroupPolicy, int64, error)
}

var (
	_ GroupPolicyRepository = (*groupPolicyRepository)(nil)
)

type groupPolicyRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewGroupPolicyRepository(db *gorm.DB) GroupPolicyRepository {
	return &groupPolicyRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "group-policy-repository"),
	}
}

func (r *groupPolicyRepository) WithTx(tx *gorm.DB) GroupPolicyRepository {
	return NewGroupPolicyRepository(tx)
}

func (r *groupPolicyRepository) Create(ctx context.Context, groupPolicy *entity.GroupPolicy) error {
	return r.db.WithContext(ctx).Create(groupPolicy).Error
}

func (r *groupPolicyRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.Node{}, id).Error
}

func (r *groupPolicyRepository) DeleteByGroupPolicyId(ctx context.Context, groupId, policyId uint64) error {
	return r.db.WithContext(ctx).Where("group_id = ? AND node_id = ?", groupId, policyId).Delete(&entity.GroupPolicy{}).Error
}

func (r *groupPolicyRepository) Update(ctx context.Context, dto *dto.GroupPolicyDto) error {
	groupPolicy := entity.GroupPolicy{}
	return r.db.WithContext(ctx).Model(&entity.GroupNode{}).Where("id = ?", dto.ID).Updates(&groupPolicy).Error
}

func (r *groupPolicyRepository) Find(ctx context.Context, id uint64) (*entity.GroupPolicy, error) {
	var groupPolicy entity.GroupPolicy
	err := r.db.WithContext(ctx).First(&groupPolicy, id).Error
	if err != nil {
		return nil, err
	}
	return &groupPolicy, nil
}

func (r *groupPolicyRepository) FindByGroupNodeId(ctx context.Context, groupId, policyId uint64) (*entity.GroupPolicy, error) {
	var groupPolicy entity.GroupPolicy
	err := r.db.WithContext(ctx).Where("group_id = ? AND node_id = ?", groupId, policyId).First(&groupPolicy).Error
	if err != nil {
		return nil, err
	}
	return &groupPolicy, nil
}

func (r *groupPolicyRepository) List(ctx context.Context, params *dto.GroupPolicyParams) ([]*entity.GroupPolicy, int64, error) {
	var (
		groupPolicies []*entity.GroupPolicy
		count         int64
		err           error
	)

	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.GroupPolicy{}))
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err := query.Find(&groupPolicies).Error; err != nil {
		return nil, 0, err
	}
	return groupPolicies, count, nil
}

func (r *groupPolicyRepository) QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, error) {
	var nodes []*entity.Node
	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.Node{}))

	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}

	return nodes, nil
}
