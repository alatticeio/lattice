package repository

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

	"gorm.io/gorm"
)

type GroupNodeRepository interface {
	WithTx(tx *gorm.DB) GroupNodeRepository
	Create(ctx context.Context, groupNode *entity.GroupNode) error
	Delete(ctx context.Context, id uint64) error
	DeleteByGroupNodeId(ctx context.Context, groupId, nodeId uint64) error
	Update(ctx context.Context, dto *dto.GroupNodeDto) error
	Find(ctx context.Context, groupNodeId uint64) (*entity.GroupNode, error)
	FindByGroupNodeId(ctx context.Context, groupId, nodeId uint64) (*entity.GroupNode, error)

	List(ctx context.Context, params *dto.GroupNodeParams) ([]*entity.GroupNode, int64, error)
}

var (
	_ GroupNodeRepository = (*groupNodeRepository)(nil)
)

type groupNodeRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewGroupNodeRepository(db *gorm.DB) GroupNodeRepository {
	return &groupNodeRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "group-member-repository"),
	}
}

func (r *groupNodeRepository) WithTx(tx *gorm.DB) GroupNodeRepository {
	return NewGroupNodeRepository(tx)
}

func (r *groupNodeRepository) Create(ctx context.Context, groupNode *entity.GroupNode) error {
	return r.db.WithContext(ctx).Create(groupNode).Error
}

func (r *groupNodeRepository) Delete(ctx context.Context, groupNodeId uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.Node{}, groupNodeId).Error
}

func (r *groupNodeRepository) DeleteByGroupNodeId(ctx context.Context, groupId, nodeId uint64) error {
	return r.db.WithContext(ctx).Where("group_id = ? AND node_id = ?", groupId, nodeId).Delete(&entity.GroupNode{}).Error
}

func (r *groupNodeRepository) Update(ctx context.Context, dto *dto.GroupNodeDto) error {
	groupNode := entity.GroupNode{}
	return r.db.WithContext(ctx).Model(&entity.GroupNode{}).Where("id = ?", dto.ID).Updates(&groupNode).Error
}

func (r *groupNodeRepository) Find(ctx context.Context, groupNodeId uint64) (*entity.GroupNode, error) {
	var groupNode entity.GroupNode
	err := r.db.WithContext(ctx).First(&groupNode, groupNodeId).Error
	if err != nil {
		return nil, err
	}
	return &groupNode, nil
}

func (r *groupNodeRepository) FindByGroupNodeId(ctx context.Context, groupId, nodeId uint64) (*entity.GroupNode, error) {
	var groupNode entity.GroupNode
	err := r.db.WithContext(ctx).Where("group_id = ? AND node_id = ?", groupId, nodeId).First(&groupNode).Error
	if err != nil {
		return nil, err
	}
	return &groupNode, nil
}

func (r *groupNodeRepository) List(ctx context.Context, params *dto.GroupNodeParams) ([]*entity.GroupNode, int64, error) {
	var (
		groupNodes []*entity.GroupNode
		count      int64
		err        error
	)

	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.GroupNode{}))

	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	//5. query
	if err := query.Find(&groupNodes).Error; err != nil {
		return nil, 0, err
	}

	return groupNodes, count, nil
}

func (r *groupNodeRepository) QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, error) {
	var nodes []*entity.Node
	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.Node{}))

	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}

	return nodes, nil
}
