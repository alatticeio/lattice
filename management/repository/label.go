package repository

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

	"gorm.io/gorm"
)

type LabelRepository interface {
	WithTx(tx *gorm.DB) LabelRepository
	Create(ctx context.Context, label *entity.Label) error
	Delete(ctx context.Context, id uint64) error
	Update(ctx context.Context, dto *dto.TagDto) error
	Find(ctx context.Context, labelId uint64) (*entity.Label, error)

	List(ctx context.Context, params *dto.LabelParams) ([]*entity.Label, int64, error)
	Query(ctx context.Context, params *dto.LabelParams) ([]*entity.Label, error)
}

var (
	_ LabelRepository = (*labelRepository)(nil)
)

type labelRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewLabelRepository(db *gorm.DB) LabelRepository {
	return &labelRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "label"),
	}
}

func (r *labelRepository) WithTx(tx *gorm.DB) LabelRepository {
	return NewLabelRepository(tx)
}

func (r *labelRepository) Create(ctx context.Context, label *entity.Label) error {
	return r.db.WithContext(ctx).Create(label).Error
}

func (r *labelRepository) Delete(ctx context.Context, groupNodeId uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.Node{}, groupNodeId).Error
}

func (r *labelRepository) Update(ctx context.Context, dto *dto.TagDto) error {
	label := &entity.Label{
		Label:     dto.Label,
		CreatedBy: dto.CreatedBy,
	}

	return r.db.WithContext(ctx).Model(&entity.Label{}).Where("id = ?", dto.ID).Updates(label).Error
}

func (r *labelRepository) Find(ctx context.Context, labelId uint64) (*entity.Label, error) {
	var label entity.Label
	err := r.db.WithContext(ctx).First(&label, labelId).Error
	if err != nil {
		return nil, err
	}
	return &label, nil
}

func (r *labelRepository) List(ctx context.Context, params *dto.LabelParams) ([]*entity.Label, int64, error) {
	var (
		labels   []*entity.Label
		count    int64
		sql      string
		wrappers []interface{}
		err      error
	)

	//1.base query
	query := r.db.WithContext(ctx).Model(&entity.Label{})

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
	if params.Page != nil {
		offset := (*params.Page - 1) * *params.Size
		query = query.Offset(offset).Limit(*params.Size)
	}

	//5. query
	if err := query.Find(&labels).Error; err != nil {
		return nil, 0, err
	}

	return labels, count, nil
}

func (r *labelRepository) Query(ctx context.Context, params *dto.LabelParams) ([]*entity.Label, error) {
	var labels []*entity.Label
	var sql string
	var wrappers []interface{}

	sql, wrappers = utils.GenerateLikeSql(params)

	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := r.db.WithContext(ctx).Model(&entity.Label{}).Where(sql, wrappers...).Find(&labels).Error; err != nil {
		return nil, err
	}

	return labels, nil
}
