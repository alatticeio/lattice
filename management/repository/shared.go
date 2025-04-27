package repository

import (
	"context"
	"errors"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

	"gorm.io/gorm"
)

type SharedRepository interface {
	WithTx(tx *gorm.DB) SharedRepository

	// node
	GetNode(ctx context.Context, id uint64) (*entity.SharedNode, error)
	GetNodeByParams(ctx context.Context, params dto.Params) (*entity.SharedNode, error)
	CreateNode(ctx context.Context, node *entity.SharedNode) error
	ListNode(ctx context.Context, params *dto.SharedNodeParams) ([]*entity.SharedNode, int64, error)
	UpdateNodes(ctx context.Context, node *entity.SharedNode, params *dto.SharedNodeParams) error

	// group
	GetGroup(ctx context.Context, id uint64) (*entity.SharedNodeGroup, error)
	GetGroupByParams(ctx context.Context, params dto.Params) (*entity.SharedNodeGroup, error)
	DeleteGroupByParams(ctx context.Context, params dto.Params) (*entity.SharedNodeGroup, error)
	CreateGroup(ctx context.Context, node *entity.SharedNodeGroup) error
	ListGroup(ctx context.Context, params *dto.SharedGroupParams) ([]*entity.SharedNodeGroup, int64, error)
	UpdateGroups(ctx context.Context, group *entity.SharedNodeGroup, params *dto.SharedGroupParams) error

	// policy
	GetPolicy(ctx context.Context, id uint64) (*entity.SharedPolicy, error)
	GetPolicyByParams(ctx context.Context, params dto.Params) (*entity.SharedPolicy, error)
	CreatePolicy(ctx context.Context, node *entity.SharedPolicy) error
	ListPolicy(ctx context.Context, params *dto.SharedPolicyParams) ([]*entity.SharedPolicy, int64, error)
	UpdatePolicy(ctx context.Context, policy *entity.SharedPolicy) error
	UpdatePolicies(ctx context.Context, policy *entity.SharedPolicy, params *dto.SharedPolicyParams) error

	// labels
	GetLabel(ctx context.Context, id uint64) (*entity.SharedLabel, error)
	GetLabelByParams(ctx context.Context, params dto.Params) (*entity.SharedLabel, error)
	CreateLabel(ctx context.Context, node *entity.SharedLabel) error
	ListLabel(ctx context.Context, params *dto.SharedLabelParams) ([]*entity.SharedLabel, int64, error)
	UpdateLabels(ctx context.Context, node *entity.SharedLabel, params *dto.SharedLabelParams) error
}

var (
	_ SharedRepository = (*sharedRepository)(nil)
)

type sharedRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func (r *sharedRepository) DeleteGroupByParams(ctx context.Context, params dto.Params) (*entity.SharedNodeGroup, error) {
	var (
		nodeGroup entity.SharedNodeGroup
	)
	sql, wrappers := utils.GenerateSql(params)
	if sql != "" {
		return &nodeGroup, r.db.WithContext(ctx).Model(&entity.SharedNodeGroup{}).Where(sql, wrappers).First(&entity.SharedNode{}).Error
	}

	return &nodeGroup, errors.New("invalid params")
}

func (r *sharedRepository) GetNodeByParams(ctx context.Context, params dto.Params) (*entity.SharedNode, error) {
	var (
		node entity.SharedNode
	)
	sql, wrappers := utils.GenerateSql(params)
	if sql != "" {
		return &node, r.db.WithContext(ctx).Model(&entity.SharedNode{}).Where(sql, wrappers).First(&entity.SharedNode{}).Error
	}

	return &node, r.db.WithContext(ctx).Model(&entity.SharedNode{}).First(&entity.SharedNode{}).Error
}

func (r *sharedRepository) GetGroupByParams(ctx context.Context, params dto.Params) (*entity.SharedNodeGroup, error) {
	var (
		nodeGroup entity.SharedNodeGroup
	)
	sql, wrappers := utils.GenerateSql(params)
	if sql != "" {
		return &nodeGroup, r.db.WithContext(ctx).Model(&entity.SharedNodeGroup{}).Where(sql, wrappers).First(&entity.SharedNode{}).Error
	}

	return &nodeGroup, r.db.WithContext(ctx).Model(&entity.SharedNodeGroup{}).First(&entity.SharedNode{}).Error
}

func (r *sharedRepository) GetPolicyByParams(ctx context.Context, params dto.Params) (*entity.SharedPolicy, error) {
	var (
		sharedPolicy entity.SharedPolicy
	)
	sql, wrappers := utils.GenerateSql(params)
	if sql != "" {
		return &sharedPolicy, r.db.WithContext(ctx).Model(&entity.SharedPolicy{}).Where(sql, wrappers).First(&entity.SharedNode{}).Error
	}

	return &sharedPolicy, r.db.WithContext(ctx).Model(&entity.SharedPolicy{}).First(&entity.SharedNode{}).Error
}

func (r *sharedRepository) GetLabelByParams(ctx context.Context, params dto.Params) (*entity.SharedLabel, error) {
	var (
		sharedLabel entity.SharedLabel
	)
	sql, wrappers := utils.GenerateSql(params)
	if sql != "" {
		return &sharedLabel, r.db.WithContext(ctx).Model(&entity.SharedLabel{}).Where(sql, wrappers).First(&entity.SharedNode{}).Error
	}

	return &sharedLabel, r.db.WithContext(ctx).Model(&entity.SharedLabel{}).First(&entity.SharedNode{}).Error
}

func (r *sharedRepository) UpdatePolicy(ctx context.Context, policy *entity.SharedPolicy) error {
	//TODO implement me
	panic("implement me")
}

func (r *sharedRepository) GetNode(ctx context.Context, id uint64) (*entity.SharedNode, error) {
	var node entity.SharedNode
	if err := r.db.WithContext(ctx).Model(&entity.SharedNode{}).Where("id = ?", id).Find(&node).Error; err != nil {
		return nil, err
	}

	return &node, nil
}

func (r *sharedRepository) GetGroup(ctx context.Context, id uint64) (*entity.SharedNodeGroup, error) {
	var sharedNodeGroup entity.SharedNodeGroup
	if err := r.db.WithContext(ctx).Model(&entity.SharedNodeGroup{}).Where("id = ?", id).Find(&sharedNodeGroup).Error; err != nil {
		return nil, err
	}

	return &sharedNodeGroup, nil
}

func (r *sharedRepository) UpdateGroups(ctx context.Context, group *entity.SharedNodeGroup, params *dto.SharedGroupParams) error {
	//TODO implement me
	panic("implement me")
}

func (r *sharedRepository) GetPolicy(ctx context.Context, id uint64) (*entity.SharedPolicy, error) {
	var sharedPolicy entity.SharedPolicy
	if err := r.db.WithContext(ctx).Model(&entity.SharedPolicy{}).Where("id = ?", id).Find(&sharedPolicy).Error; err != nil {
		return nil, err
	}

	return &sharedPolicy, nil
}

func (r *sharedRepository) UpdatePolicies(ctx context.Context, policy *entity.SharedPolicy, params *dto.SharedPolicyParams) error {
	//TODO implement me
	panic("implement me")
}

func (r *sharedRepository) GetLabel(ctx context.Context, id uint64) (*entity.SharedLabel, error) {
	var sharedLabel entity.SharedLabel
	if err := r.db.WithContext(ctx).Model(&entity.SharedLabel{}).Where("id = ?", id).Find(&sharedLabel).Error; err != nil {
		return nil, err
	}

	return &sharedLabel, nil
}

func (r *sharedRepository) UpdateLabels(ctx context.Context, node *entity.SharedLabel, params *dto.SharedLabelParams) error {
	//TODO implement me
	panic("implement me")
}

func (r *sharedRepository) UpdateNodes(ctx context.Context, node *entity.SharedNode, params *dto.SharedNodeParams) error {
	sql, wrappers := utils.Generate(params)

	if sql != "" {
		return r.db.WithContext(ctx).Model(node).Where(sql, wrappers).Updates(node).Error
	}

	return r.db.WithContext(ctx).Model(node).Updates(node).Error
}

func NewSharedRepository(db *gorm.DB) SharedRepository {
	return &sharedRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "shared-repository"),
	}
}

func (r *sharedRepository) WithTx(tx *gorm.DB) SharedRepository {
	return NewSharedRepository(tx)
}

func (r *sharedRepository) CreateNode(ctx context.Context, node *entity.SharedNode) error {
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *sharedRepository) ListNode(ctx context.Context, params *dto.SharedNodeParams) ([]*entity.SharedNode, int64, error) {
	var (
		err   error
		nodes []*entity.SharedNode
		count int64
	)
	sql, wrappers := utils.Generate(params)
	query := r.db.WithContext(ctx).Model(&entity.SharedNode{}).Preload("NodeLabels")

	if sql != "" {
		query = query.Where(sql, wrappers)
	}

	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	err = query.Find(&nodes).Error
	return nodes, count, err
}

// group
func (r *sharedRepository) CreateGroup(ctx context.Context, node *entity.SharedNodeGroup) error {
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *sharedRepository) ListGroup(ctx context.Context, params *dto.SharedGroupParams) ([]*entity.SharedNodeGroup, int64, error) {
	var (
		err   error
		nodes []*entity.SharedNodeGroup
		count int64
	)
	sql, wrappers := utils.Generate(params)
	query := r.db.WithContext(ctx).Model(&entity.SharedNodeGroup{}).Preload("Groups")

	if sql != "" {
		query = query.Where(sql, wrappers)
	}

	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	err = query.Find(&nodes).Error
	return nodes, count, err
}

// policy
func (r *sharedRepository) CreatePolicy(ctx context.Context, node *entity.SharedPolicy) error {
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *sharedRepository) ListPolicy(ctx context.Context, params *dto.SharedPolicyParams) ([]*entity.SharedPolicy, int64, error) {
	var (
		err   error
		nodes []*entity.SharedPolicy
		count int64
	)
	sql, wrappers := utils.Generate(params)
	query := r.db.WithContext(ctx).Model(&entity.SharedPolicy{}).Preload("Groups")

	if sql != "" {
		query = query.Where(sql, wrappers)
	}

	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	err = query.Find(&nodes).Error
	return nodes, count, err
}

// label
func (r *sharedRepository) CreateLabel(ctx context.Context, node *entity.SharedLabel) error {
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *sharedRepository) ListLabel(ctx context.Context, params *dto.SharedLabelParams) ([]*entity.SharedLabel, int64, error) {
	var (
		err   error
		nodes []*entity.SharedLabel
		count int64
	)
	sql, wrappers := utils.Generate(params)
	query := r.db.WithContext(ctx).Model(&entity.SharedLabel{}).Preload("Groups")

	if sql != "" {
		query = query.Where(sql, wrappers)
	}

	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	err = query.Find(&nodes).Error
	return nodes, count, err
}
