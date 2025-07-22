package repository

import (
	"context"
	"gorm.io/gorm"
	"linkany/management/entity"
	"linkany/pkg/log"
)

type NodeToRepository interface {
	WithTx(tx *gorm.DB) NodeToRepository
	Create(ctx context.Context, nodeTo *entity.NodeTo) error
	Delete(ctx context.Context, id uint64) error
	DeleteAllByNodeId(ctx context.Context, nodeId uint64) error
	DeleteByNodeToId(ctx context.Context, nodeToId uint64) error
	Find(ctx context.Context, id uint64) (*entity.NodeTo, error)

	//List(ctx context.Context, params *dto.NodeLabelParams) ([]*entity.NodeLabel, int64, error)
	//Query(ctx context.Context, params *dto.NodeLabelParams) ([]*entity.NodeLabel, error)
}

var (
	_ NodeToRepository = (*nodeToRepository)(nil)
)

type nodeToRepository struct {
	db       *gorm.DB
	logger   *log.Logger
	baseRepo BaseRepository[entity.NodeTo]
}

func NewNodeToRepository(db *gorm.DB) NodeToRepository {
	return &nodeToRepository{
		db:       db,
		logger:   log.NewLogger(log.Loglevel, "node-to-repository"),
		baseRepo: NewNodeBaseRepository[entity.NodeTo](db),
	}
}

func (r nodeToRepository) WithTx(tx *gorm.DB) NodeToRepository {
	return NewNodeToRepository(tx)
}

func (r nodeToRepository) Create(ctx context.Context, nodeTo *entity.NodeTo) error {
	return r.db.WithContext(ctx).Create(nodeTo).Error
}

func (r nodeToRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.NodeTo{}, id).Error
}

func (r nodeToRepository) DeleteAllByNodeId(ctx context.Context, nodeId uint64) error {
	return r.db.WithContext(ctx).Where("node_id = ?", nodeId).Delete(&entity.NodeTo{}).Error
}

func (r nodeToRepository) DeleteByNodeToId(ctx context.Context, nodeToId uint64) error {
	return r.db.WithContext(ctx).Where("node_to_id = ?", nodeToId).Delete(&entity.NodeTo{}).Error
}

func (r nodeToRepository) Find(ctx context.Context, id uint64) (*entity.NodeTo, error) {
	var nodeTo entity.NodeTo
	err := r.db.WithContext(ctx).First(&nodeTo, id).Error
	if err != nil {
		return nil, err
	}
	return &nodeTo, nil
}
