package repository

import (
	"context"
	"gorm.io/gorm"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/management/utils"
	"wireflow/pkg/log"
)

type NodeRepository interface {
	WithTx(tx *gorm.DB) NodeRepository
	Create(ctx context.Context, node *entity.Node) error
	Delete(ctx context.Context, nodeId uint64) error
	DeleteByAppId(ctx context.Context, appId string) error
	Update(ctx context.Context, node *dto.NodeDto) error
	UpdateStatus(ctx context.Context, nodeDto *dto.NodeDto) error
	Find(ctx context.Context, nodeId uint64) (*entity.Node, error)
	FindIn(ctx context.Context, nodeIds []uint64) ([]*entity.Node, error)
	FindByAppId(ctx context.Context, appId string) (*entity.Node, error)

	ListNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, int64, error)
	QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, error)

	GetAddress() int64
}

var (
	_ NodeRepository = (*nodeRepository)(nil)
)

type nodeRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewNodeRepository(db *gorm.DB) NodeRepository {
	return &nodeRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "node-repository"),
	}
}

func (r *nodeRepository) WithTx(tx *gorm.DB) NodeRepository {
	return &nodeRepository{
		db: tx,
	}
}

func (r *nodeRepository) Create(ctx context.Context, node *entity.Node) error {
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *nodeRepository) Delete(ctx context.Context, nodeId uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.Node{}, nodeId).Error
}

func (r *nodeRepository) DeleteByAppId(ctx context.Context, appId string) error {
	return r.db.WithContext(ctx).Where("app_id = ?", appId).Delete(&entity.Node{}).Error
}

func (r *nodeRepository) Update(ctx context.Context, nodeDto *dto.NodeDto) error {
	return r.db.WithContext(ctx).Model(&entity.Node{}).Where("id = ?", nodeDto.ID).Updates(map[string]interface{}{
		"active_status": nodeDto.ActiveStatus,
		"connect_type":  nodeDto.ConnectType,
		"name":          nodeDto.Name,
	}).Error
}

func (r *nodeRepository) UpdateStatus(ctx context.Context, nodeDto *dto.NodeDto) error {
	return r.db.WithContext(ctx).Model(&entity.Node{}).Where("public_key = ?", nodeDto.PublicKey).Updates(map[string]interface{}{
		"status": nodeDto.Status,
	}).Error
}

func (r *nodeRepository) Find(ctx context.Context, nodeId uint64) (*entity.Node, error) {
	var node entity.Node
	err := r.db.WithContext(ctx).First(&node, nodeId).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *nodeRepository) FindIn(ctx context.Context, nodeIds []uint64) ([]*entity.Node, error) {
	var nodes []*entity.Node
	err := r.db.WithContext(ctx).Where("id IN ?", nodeIds).Find(&nodes).Error
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *nodeRepository) FindByAppId(ctx context.Context, appId string) (*entity.Node, error) {
	var node *entity.Node
	err := r.db.WithContext(ctx).Where("app_id = ?", appId).Find(&node).Error
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (r *nodeRepository) ListNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, int64, error) {
	var (
		nodes []*entity.Node
		count int64
		err   error
	)

	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.Node{}).Preload("NodeLabels").Preload("Group"))
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err := query.Find(&nodes).Error; err != nil {
		return nil, 0, err
	}

	return nodes, count, nil
}

func (r *nodeRepository) QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, error) {
	var nodes []*entity.Node
	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.Node{}))

	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}

	return nodes, nil
}

func (r *nodeRepository) GetAddress() int64 {
	var count int64
	if err := r.db.Model(&entity.Node{}).Count(&count).Error; err != nil {
		r.logger.Errorf("err： %s", err.Error())
		return -1
	}
	if count > 253 {
		return -1
	}
	return count
}
