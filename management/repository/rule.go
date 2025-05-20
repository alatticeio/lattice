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
	return r.db.WithContext(ctx).Model(&entity.AccessRule{}).Where("id=?", ruleDto.ID).Updates(map[string]interface{}{
		"own_id":      utils.GetUserIdFromCtx(ctx),
		"policy_id":   ruleDto.PolicyID,
		"source_type": ruleDto.SourceType,
		"source_id":   ruleDto.SourceID,
		"target_type": ruleDto.TargetType,
		"target_id":   ruleDto.TargetID,
		"actions":     ruleDto.Actions,
		"status":      ruleDto.Status,
		"conditions":  string(data),
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
		rules []*entity.AccessRule
		count int64
		err   error
	)

	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.AccessRule{}))
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}
	if err := query.Find(&rules).Error; err != nil {
		return nil, 0, err
	}

	return rules, count, nil
}

func (r *ruleRepository) Query(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*entity.AccessRule, error) {
	var rules []*entity.AccessRule
	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.AccessRule{}))

	if err := query.Find(&rules).Error; err != nil {
		return nil, err
	}

	return rules, nil
}
