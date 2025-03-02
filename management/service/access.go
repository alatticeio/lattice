package service

import (
	"context"
	"encoding/json"
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type AccessPolicyService interface {
	// Policy manage
	CreatePolicy(ctx context.Context, policyDto *dto.AccessPolicyDto) error
	UpdatePolicy(ctx context.Context, policyDto *dto.AccessPolicyDto) error
	DeletePolicy(ctx context.Context, policyID uint) error
	GetPolicy(ctx context.Context, policyID uint) (*entity.AccessPolicy, error)
	ListGroupPolicies(ctx context.Context, params *dto.AccessPolicyParams) (*vo.PageVo, error)

	// Rule manage
	AddRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error
	UpdateRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error
	DeleteRule(ctx context.Context, ruleID uint) error
	ListPolicyRules(ctx context.Context, params *dto.AccessPolicyRuleParams) (*vo.PageVo, error)

	// Access control
	CheckAccess(ctx context.Context, sourceNodeID, targetNodeID uint, action string) (bool, error)
	BatchCheckAccess(ctx context.Context, requests []AccessRequest) ([]AccessResult, error)

	// Audit log
	GetAccessLogs(ctx context.Context, filter AccessLogFilter) ([]entity.AccessLog, error)
}

// 访问请求结构
type AccessRequest struct {
	SourceNodeID uint   `json:"source_node_id"`
	TargetNodeID uint   `json:"target_node_id"`
	Action       string `json:"action"`
}

// 访问结果结构
type AccessResult struct {
	Allowed  bool   `json:"allowed"`
	PolicyID uint   `json:"policy_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type AccessLogFilter struct {
	SourceNodeID uint `json:"source_node_id,omitempty"`
	TargetNodeID uint `json:"target_node_id,omitempty"`
}

var (
	_ AccessPolicyService = (*accessPolicyServiceImpl)(nil)
)

type accessPolicyServiceImpl struct {
	logger *log.Logger
	*DatabaseService
}

func (a accessPolicyServiceImpl) CreatePolicy(ctx context.Context, policyDto *dto.AccessPolicyDto) error {
	var count int64
	if err := a.Model(&entity.AccessPolicy{}).Where("name = ? and groupd_id = ?", policyDto.Name, policyDto.GroupID).First(&entity.AccessPolicy{}).Count(&count).Error; err != nil {
		return nil
	}

	if count > 0 {
		return fmt.Errorf("policy name %s already exists", policyDto.Name)
	}

	return a.Create(&entity.AccessPolicy{
		Name:        policyDto.Name,
		GroupID:     policyDto.GroupID,
		Priority:    policyDto.Priority,
		Effect:      policyDto.Effect,
		Description: policyDto.Description,
		Status:      policyDto.Status,
		CreatedBy:   policyDto.CreatedBy,
		UpdatedBy:   policyDto.UpdatedBy,
	}).Error
}

func (a accessPolicyServiceImpl) UpdatePolicy(ctx context.Context, policyDto *dto.AccessPolicyDto) error {
	return a.Where("id = ?", policyDto.ID).Save(&entity.AccessPolicy{
		Name:        policyDto.Name,
		GroupID:     policyDto.GroupID,
		Priority:    policyDto.Priority,
		Effect:      policyDto.Effect,
		Description: policyDto.Description,
		Status:      policyDto.Status,
		CreatedBy:   policyDto.CreatedBy,
		UpdatedBy:   policyDto.UpdatedBy,
	}).Error
}

func (a accessPolicyServiceImpl) DeletePolicy(ctx context.Context, policyID uint) error {
	return a.Where("id = ?", policyID).Delete(&entity.AccessPolicy{}).Error
}

func (a accessPolicyServiceImpl) GetPolicy(ctx context.Context, policyID uint) (*entity.AccessPolicy, error) {
	var policy entity.AccessPolicy
	err := a.Where("id = ?", policyID).Find(&policy).Error
	return &policy, err
}

func (a accessPolicyServiceImpl) ListGroupPolicies(ctx context.Context, params *dto.AccessPolicyParams) (*vo.PageVo, error) {
	var policies []vo.AccessPolicyVo
	var result = new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := a.DB
	if sql != "" {
		db = db.Model(&entity.AccessPolicy{}).Where(sql, wrappers)
	}

	if err := db.Model(&entity.AccessPolicy{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	err := db.Model(&entity.AccessPolicy{}).Offset((params.PageNo - 1) * params.PageSize).Limit(params.PageSize).Find(&policies).Error

	result.Data = policies
	result.Current = params.PageNo
	result.PageNo = params.PageNo
	result.PageSize = params.PageSize
	return result, err
}

func (a accessPolicyServiceImpl) AddRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error {
	data, err := json.Marshal(ruleDto.Conditions)
	if err != nil {
		return err
	}
	return a.Create(&entity.AccessRule{
		PolicyID:   ruleDto.PolicyID,
		SourceType: ruleDto.SourceType,
		SourceID:   ruleDto.SourceID,
		TargetType: ruleDto.TargetType,
		TargetID:   ruleDto.TargetID,
		Actions:    ruleDto.Actions,
		Conditions: string(data),
	}).Error
}

func (a accessPolicyServiceImpl) UpdateRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error {
	data, err := json.Marshal(ruleDto.Conditions)
	if err != nil {
		return err
	}
	return a.Where("id = ?", ruleDto.ID).Save(&entity.AccessRule{
		PolicyID:   ruleDto.PolicyID,
		SourceType: ruleDto.SourceType,
		SourceID:   ruleDto.SourceID,
		TargetType: ruleDto.TargetType,
		TargetID:   ruleDto.TargetID,
		Actions:    ruleDto.Actions,
		Conditions: string(data),
	}).Error

}

func (a accessPolicyServiceImpl) DeleteRule(ctx context.Context, ruleID uint) error {
	return a.Where("id = ?", ruleID).Delete(&entity.AccessRule{}).Error
}

func (a accessPolicyServiceImpl) ListPolicyRules(ctx context.Context, params *dto.AccessPolicyRuleParams) (*vo.PageVo, error) {
	var policies []vo.AccessRuleVo
	var result = new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := a.DB
	if sql != "" {
		db = db.Model(&entity.AccessRule{}).Where(sql, wrappers)
	}

	if err := db.Model(&entity.AccessRule{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := db.Model(&entity.AccessRule{}).Offset((params.PageNo - 1) * params.PageSize).Limit(params.PageSize).Find(&policies).Error; err != nil {
		return nil, err
	}

	result.Data = policies
	result.Current = params.PageNo
	result.PageNo = params.PageNo
	result.PageSize = params.PageSize
	return result, nil
}

func (a accessPolicyServiceImpl) CheckAccess(ctx context.Context, sourceNodeID, targetNodeID uint, action string) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (a accessPolicyServiceImpl) BatchCheckAccess(ctx context.Context, requests []AccessRequest) ([]AccessResult, error) {
	//TODO implement me
	panic("implement me")
}

func (a accessPolicyServiceImpl) GetAccessLogs(ctx context.Context, filter AccessLogFilter) ([]entity.AccessLog, error) {
	//TODO implement me
	panic("implement me")
}

func NewAccessPolicyService(db *DatabaseService) AccessPolicyService {
	return &accessPolicyServiceImpl{
		logger:          log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "access_policy_service")),
		DatabaseService: db,
	}
}
