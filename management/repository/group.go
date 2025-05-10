package repository

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

	"gorm.io/gorm"
)

type GroupRepository interface {
	WithTx(tx *gorm.DB) GroupRepository
	Create(ctx context.Context, group *entity.NodeGroup) error
	Delete(ctx context.Context, id uint64) error
	Update(ctx context.Context, dto *dto.NodeGroupDto) (*entity.NodeGroup, error)
	Find(ctx context.Context, id uint64) (*entity.NodeGroup, error)
	FindByName(ctx context.Context, name string) (*entity.NodeGroup, error)

	List(ctx context.Context, params *dto.GroupParams) ([]*entity.NodeGroup, int64, error)
	Query(ctx context.Context, params *dto.GroupParams) ([]*entity.NodeGroup, error)
}

var (
	_ GroupRepository = (*groupRepository)(nil)
)

type groupRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewGroupRepository(db *gorm.DB) GroupRepository {
	return &groupRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "group-member-repository"),
	}
}

func (r *groupRepository) WithTx(tx *gorm.DB) GroupRepository {
	return NewGroupRepository(tx)
}

func (r *groupRepository) Create(ctx context.Context, group *entity.NodeGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *groupRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.NodeGroup{}, id).Error
}

func (r *groupRepository) Update(ctx context.Context, dto *dto.NodeGroupDto) (*entity.NodeGroup, error) {
	group := entity.NodeGroup{}
	return &group, r.db.WithContext(ctx).Model(&entity.NodeGroup{}).Where("id = ?", dto.ID).Updates(&group).Find(&group).Error
}

func (r *groupRepository) Find(ctx context.Context, id uint64) (*entity.NodeGroup, error) {
	var group entity.NodeGroup
	err := r.db.WithContext(ctx).First(&group, id).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *groupRepository) FindByName(ctx context.Context, name string) (*entity.NodeGroup, error) {
	var group entity.NodeGroup
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&group).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *groupRepository) List(ctx context.Context, params *dto.GroupParams) ([]*entity.NodeGroup, int64, error) {
	var (
		groups   []*entity.NodeGroup
		count    int64
		sql      string
		wrappers []interface{}
		err      error
	)

	//1.base query
	query := r.db.WithContext(ctx).Model(&entity.NodeGroup{}).Preload("GroupNodes").Preload("GroupPolicies")

	sql, wrappers = utils.Generate(params)
	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)

	//2. add filter params
	if wrappers != nil {
		query = query.Where(sql, wrappers)
	}

	//3.got total
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	//4. add pagination
	pageOffset := params.GetPageOffset()
	if params.Page != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	//5. query
	if err := query.Find(&groups).Error; err != nil {
		return nil, 0, err
	}

	return groups, count, nil
}

func (r *groupRepository) Query(ctx context.Context, params *dto.GroupParams) ([]*entity.NodeGroup, error) {
	var groups []*entity.NodeGroup
	var sql string
	var wrappers []interface{}

	sql, wrappers = utils.Generate(params)

	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := r.db.WithContext(ctx).Where(sql, wrappers...).Find(&groups).Error; err != nil {
		return nil, err
	}

	return groups, nil
}
