package repository

import (
	"context"
	"encoding/json"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

	"gorm.io/gorm"
)

type RuleRepository interface {
	WithTx(tx *gorm.DB) RuleRepository
	Create(ctx context.Context, groupPolicy *entity.AccessRule) error
	CreateRuleRel(ctx context.Context, ruleRel *entity.AccessRuleRel) error
	Delete(ctx context.Context, id uint64) error
	Update(ctx context.Context, dto *dto.AccessRuleDto) error
	Find(ctx context.Context, id uint64) (*entity.AccessRule, error)

	List(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*entity.AccessRule, int64, error)
	Query(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*entity.AccessRule, error)
}

var (
	_ RuleRepository = (*ruleRepository)(nil)
)

type ruleRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewRuleRepository(db *gorm.DB) RuleRepository {
	return &ruleRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "rule-repository"),
	}
}

func (r *ruleRepository) WithTx(tx *gorm.DB) RuleRepository {
	return NewRuleRepository(tx)
}

func (r *ruleRepository) Create(ctx context.Context, rule *entity.AccessRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *ruleRepository) CreateRuleRel(ctx context.Context, ruleRel *entity.AccessRuleRel) error {
	return r.db.WithContext(ctx).Create(ruleRel).Error
}

func (r *ruleRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.AccessRule{}, id).Error
}

func (r *ruleRepository) Update(ctx context.Context, ruleDto *dto.AccessRuleDto) error {
	data, err := json.Marshal(ruleDto.Conditions)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Updates(&entity.AccessRule{
		PolicyId:   ruleDto.PolicyID,
		SourceType: ruleDto.SourceType,
		SourceId:   ruleDto.SourceID,
		TargetType: ruleDto.TargetType,
		TargetId:   ruleDto.TargetID,
		Actions:    ruleDto.Actions,
		Conditions: string(data),
	}).Error

}

func (r *ruleRepository) Find(ctx context.Context, id uint64) (*entity.AccessRule, error) {
	var rule entity.AccessRule
	err := r.db.WithContext(ctx).First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *ruleRepository) List(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*entity.AccessRule, int64, error) {
	var (
		rules    []*entity.AccessRule
		count    int64
		sql      string
		wrappers []interface{}
		err      error
	)

	//1.base query
	query := r.db.WithContext(ctx).Model(&entity.AccessRule{})

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
	if err := query.Find(&rules).Error; err != nil {
		return nil, 0, err
	}

	return rules, count, nil
}

func (r *ruleRepository) Query(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*entity.AccessRule, error) {
	var rules []*entity.AccessRule
	var sql string
	var wrappers []interface{}

	sql, wrappers = utils.Generate(params)

	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := r.db.WithContext(ctx).Where(sql, wrappers...).Find(&rules).Error; err != nil {
		return nil, err
	}

	return rules, nil
}
