package repository

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

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
		sql        string
		wrappers   []interface{}
		err        error
	)

	//1.base query
	query := r.db.WithContext(ctx).Model(&entity.NodeLabel{})

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
	if err := query.Find(&nodeLabels).Error; err != nil {
		return nil, 0, err
	}

	return nodeLabels, count, nil
}

func (r *nodeLabelRepository) Query(ctx context.Context, params *dto.NodeLabelParams) ([]*entity.NodeLabel, error) {
	var nodeLabels []*entity.NodeLabel
	var sql string
	var wrappers []interface{}

	sql, wrappers = utils.Generate(params)

	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := r.db.WithContext(ctx).Where(sql, wrappers...).Find(&nodeLabels).Error; err != nil {
		return nil, err
	}

	return nodeLabels, nil
}
