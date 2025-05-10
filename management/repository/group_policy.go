package repository

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

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
		sql           string
		wrappers      []interface{}
		err           error
	)

	//1.base query
	query := r.db.WithContext(ctx).Model(&entity.GroupPolicy{})

	sql, wrappers = utils.Generate(params)
	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)

	//2. add filter params
	query = query.Where(sql, wrappers)

	//3.got total
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	//4. add pagination
	if params.Page != nil {
		offset := (*params.Size - 1) * *params.Size
		query = query.Offset(offset).Limit(*params.Size)
	}

	//5. query
	if err := query.Find(&groupPolicies).Error; err != nil {
		return nil, 0, err
	}

	return groupPolicies, count, nil
}

func (r *groupPolicyRepository) QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, error) {
	var nodes []*entity.Node
	var sql string
	var wrappers []interface{}

	if params.Keyword != nil {
		sql, wrappers = utils.GenerateLikeSql(params)
	} else {
		sql, wrappers = utils.Generate(params)
	}

	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := r.db.WithContext(ctx).Where(sql, wrappers...).Find(&nodes).Error; err != nil {
		return nil, err
	}

	return nodes, nil
}
