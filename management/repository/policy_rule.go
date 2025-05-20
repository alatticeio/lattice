package repository

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

	"gorm.io/gorm"
)

type PolicyRuleRepository interface {
	WithTx(tx *gorm.DB) PolicyRuleRepository
	Create(ctx context.Context, groupPolicy *entity.AccessRule) error
	Delete(ctx context.Context, id uint64) error
	Update(ctx context.Context, dto *dto.AccessRuleDto) error
	Find(ctx context.Context, id uint64) (*entity.AccessRule, error)

	List(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*entity.AccessRule, int64, error)
	Query(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*entity.AccessRule, error)
}

var (
	_ PolicyRuleRepository = (*policyRuleRepository)(nil)
)

type policyRuleRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewPolicyRuleRepository(db *gorm.DB) PolicyRuleRepository {
	return &policyRuleRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "rule-repository"),
	}
}

func (r *policyRuleRepository) WithTx(tx *gorm.DB) PolicyRuleRepository {
	return NewPolicyRuleRepository(tx)
}

func (r *policyRuleRepository) Create(ctx context.Context, rule *entity.AccessRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *policyRuleRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.AccessRule{}, id).Error
}

func (r *policyRuleRepository) Update(ctx context.Context, dto *dto.AccessRuleDto) error {
	rule := entity.AccessRule{}
	return r.db.WithContext(ctx).Model(&entity.GroupNode{}).Where("id = ?", dto.ID).Updates(&rule).Error
}

func (r *policyRuleRepository) Find(ctx context.Context, id uint64) (*entity.AccessRule, error) {
	var rule entity.AccessRule
	err := r.db.WithContext(ctx).First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *policyRuleRepository) List(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*entity.AccessRule, int64, error) {
	var (
		rules []*entity.AccessRule
		count int64
		err   error
	)

	//1.base query
	query := r.db.WithContext(ctx).Model(&entity.AccessRule{}).Preload("SourceNode", func(db *gorm.DB) *gorm.DB {
		// 使用子查询，根据主表的 source_type 进行过滤
		return db.Where("EXISTS (SELECT 1 FROM la_access_rule WHERE "+
			"la_access_rule.source_id = la_node.id AND "+
			"la_access_rule.source_type = ?)", utils.Node.String())
	}).Preload("TargetNode", func(db *gorm.DB) *gorm.DB {
		// 使用子查询，根据主表的 source_type 进行过滤
		return db.Where("EXISTS (SELECT 1 FROM la_access_rule WHERE "+
			"la_access_rule.target_id = la_node.id AND "+
			"la_access_rule.target_type = ?)", utils.Node.String())
	}).Preload("SourceLabel", func(db *gorm.DB) *gorm.DB {
		// 使用子查询，根据主表的 source_type 进行过滤
		return db.Where("EXISTS (SELECT 1 FROM la_access_rule WHERE "+
			"la_access_rule.source_id = la_label.id AND "+
			"la_access_rule.source_type = ?)", utils.Label.String())
	}).Preload("TargetLabel", func(db *gorm.DB) *gorm.DB {
		// 使用子查询，根据主表的 source_type 进行过滤
		return db.Where("EXISTS (SELECT 1 FROM la_access_rule WHERE "+
			"la_access_rule.target_id = la_label.id AND "+
			"la_access_rule.target_type = ?)", utils.Label.String())
	})

	conditions := utils.GenerateQuery(params, false)
	realQuery := conditions.BuildQuery(query)

	if err = realQuery.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if params.Page != nil {
		realQuery.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err := realQuery.Find(&rules).Error; err != nil {
		return nil, 0, err
	}

	return rules, count, nil
}

func (r *policyRuleRepository) Query(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*entity.AccessRule, error) {
	var rules []*entity.AccessRule
	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.AccessRule{}))
	if err := query.Find(&rules).Error; err != nil {
		return nil, err
	}

	return rules, nil
}
