package repository

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

	"gorm.io/gorm"
)

type NodeRepository interface {
	WithTx(tx *gorm.DB) NodeRepository
	Create(ctx context.Context, node *entity.Node) error
	Delete(ctx context.Context, nodeId uint64) error
	DeleteByAppId(ctx context.Context, appId string) error
	Update(ctx context.Context, node *entity.Node) error
	Find(ctx context.Context, nodeId uint64) (*entity.Node, error)
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
		db: db,
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

func (r *nodeRepository) Update(ctx context.Context, e *entity.Node) error {
	var node entity.Node
	if err := r.db.WithContext(ctx).Where("public_key = ?", node.PublicKey).First(&node).Error; err != nil {
		return err
	}
	node.Status = e.Status

	return r.db.WithContext(ctx).Save(node).Error
}

func (r *nodeRepository) Find(ctx context.Context, nodeId uint64) (*entity.Node, error) {
	var node entity.Node
	err := r.db.WithContext(ctx).First(&node, nodeId).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
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
		nodes    []*entity.Node
		count    int64
		sql      string
		wrappers []interface{}
		err      error
	)

	//1.base query
	query := r.db.WithContext(ctx).Model(&entity.Node{}).Preload("NodeLabels").Preload("Group")

	if params.Keyword != nil {
		sql, wrappers = utils.GenerateSql(params)
	} else {
		sql, wrappers = utils.Generate(params)
	}
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
	if err := query.Find(&nodes).Error; err != nil {
		return nil, 0, err
	}

	return nodes, count, nil
}

func (r *nodeRepository) QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, error) {
	var nodes []*entity.Node
	var sql string
	var wrappers []interface{}

	if params.Keyword != nil {
		sql, wrappers = utils.GenerateSql(params)
	} else {
		sql, wrappers = utils.Generate(params)
	}

	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := r.db.WithContext(ctx).Where(sql, wrappers...).Find(&nodes).Error; err != nil {
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
