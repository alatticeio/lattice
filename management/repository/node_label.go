package repository

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/pkg/log"
	"wireflow/pkg/utils"

	"gorm.io/gorm"
)

type NodeLabelRepository interface {
	WithTx(tx *gorm.DB) NodeLabelRepository
	Create(ctx context.Context, nodeLabel *entity.NodeLabel) error
	Delete(ctx context.Context, id uint64) error
	DeleteByLabelId(ctx context.Context, nodeId, labelId uint64) error
	Update(ctx context.Context, dto *dto.NodeLabelDto) error
	Find(ctx context.Context, id uint64) (*entity.NodeLabel, error)

	List(ctx context.Context, params *dto.NodeLabelParams) ([]*entity.NodeLabel, int64, error)
	Query(ctx context.Context, params *dto.NodeLabelParams) ([]*entity.NodeLabel, error)
}

var (
	_ NodeLabelRepository = (*nodeLabelRepository)(nil)
)

type nodeLabelRepository struct {
	db       *gorm.DB
	logger   *log.Logger
	baseRepo BaseRepository[entity.NodeLabel]
}

func NewNodeLabelRepository(db *gorm.DB) NodeLabelRepository {
	return &nodeLabelRepository{
		db:       db,
		logger:   log.NewLogger(log.Loglevel, "group-member-repository"),
		baseRepo: NewNodeBaseRepository[entity.NodeLabel](db),
	}
}

func (r *nodeLabelRepository) WithTx(tx *gorm.DB) NodeLabelRepository {
	return NewNodeLabelRepository(tx)
}

func (r *nodeLabelRepository) Create(ctx context.Context, nodeLabel *entity.NodeLabel) error {
	return r.db.WithContext(ctx).Create(nodeLabel).Error
}

func (r *nodeLabelRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.Node{}, id).Error
}

func (r *nodeLabelRepository) DeleteByLabelId(ctx context.Context, nodeId, labelId uint64) error {
	return r.db.WithContext(ctx).Where("node_id = ? and label_id = ?", nodeId, labelId).Delete(&entity.NodeLabel{}).Error
}

func (r *nodeLabelRepository) Update(ctx context.Context, dto *dto.NodeLabelDto) error {
	nodeLabel := entity.NodeLabel{
		LabelId:   dto.LabelID,
		LabelName: dto.LabelName,
	}
	return r.db.WithContext(ctx).Model(&entity.GroupNode{}).Where("id = ?", dto.ID).Updates(&nodeLabel).Error
}

func (r *nodeLabelRepository) Find(ctx context.Context, id uint64) (*entity.NodeLabel, error) {
	var nodeLabel entity.NodeLabel
	err := r.db.WithContext(ctx).Model(&entity.NodeLabel{}).First(&nodeLabel, id).Error
	if err != nil {
		return nil, err
	}
	return &nodeLabel, nil
}

func (r *nodeLabelRepository) List(ctx context.Context, params *dto.NodeLabelParams) ([]*entity.NodeLabel, int64, error) {
	var (
		nodeLabels []*entity.NodeLabel
		count      int64
		err        error
	)

	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.NodeLabel{}))
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err := query.Find(&nodeLabels).Error; err != nil {
		return nil, 0, err
	}

	return nodeLabels, count, nil
}

func (r *nodeLabelRepository) Query(ctx context.Context, params *dto.NodeLabelParams) ([]*entity.NodeLabel, error) {
	var nodeLabels []*entity.NodeLabel
	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.NodeLabel{}))
	if err := query.Find(&nodeLabels).Error; err != nil {
		return nil, err
	}

	return nodeLabels, nil
}
