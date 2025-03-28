package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
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

	//ListPagePolicies list with query
	QueryPolicies(ctx context.Context, params *dto.AccessPolicyParams) ([]*vo.AccessPolicyVo, error)
	DeleteUserResourcePermission(ctx context.Context, inviteId, permissionId uint) error

	// Rule manage
	AddRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error
	GetRule(ctx context.Context, id int64) (vo.AccessRuleVo, error)
	UpdateRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error
	DeleteRule(ctx context.Context, ruleID uint) error
	ListPolicyRules(ctx context.Context, params *dto.AccessPolicyRuleParams) (*vo.PageVo, error)

	// Access control
	CheckAccess(ctx context.Context, resourceType utils.ResourceType, resourceId uint, action string) (bool, error)
	BatchCheckAccess(ctx context.Context, requests []AccessRequest) ([]AccessResult, error)

	// Audit log
	GetAccessLogs(ctx context.Context, filter AccessLogFilter) ([]entity.AccessLog, error)

	// Permissions
	ListPermissions(ctx context.Context, params *dto.PermissionParams) (*vo.PageVo, error)

	// Permissions
	QueryPermissions(ctx context.Context, params *dto.PermissionParams) ([]*vo.PermissionVo, error)
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
	if err := a.Model(&entity.AccessPolicy{}).Where("name = ? and group_id = ?", policyDto.Name, policyDto.GroupID).Count(&count).Error; err != nil {
		return err
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

	err := db.Model(&entity.AccessPolicy{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&policies).Error

	result.Data = policies
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size
	return result, err
}

func (a accessPolicyServiceImpl) QueryPolicies(ctx context.Context, params *dto.AccessPolicyParams) ([]*vo.AccessPolicyVo, error) {
	var policies []*vo.AccessPolicyVo
	sql, wrappers := utils.GenerateSql(params)

	if sql != "" {
		err := a.Model(&entity.AccessPolicy{}).Where(sql, wrappers).Find(&policies).Error
		return policies, err
	}

	return policies, nil
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

func (a accessPolicyServiceImpl) GetRule(ctx context.Context, ruleId int64) (vo.AccessRuleVo, error) {
	var rule entity.AccessRule
	err := a.Where("id = ?", ruleId).Find(&rule).Error
	return vo.AccessRuleVo{
		ID:         rule.ID,
		RuleType:   rule.RuleType,
		PolicyID:   rule.PolicyID,
		SourceType: rule.SourceType,
		SourceID:   rule.SourceID,
		TargetType: rule.TargetType,
		TargetID:   rule.TargetID,
		Actions:    rule.Actions,
		TimeType:   rule.TimeType,
		Conditions: rule.Conditions,
		CreatedAt:  rule.CreatedAt,
		UpdatedAt:  rule.UpdatedAt,
	}, err
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
	return a.Model(&entity.AccessRule{}).Where("id = ?", ruleID).Delete(&entity.AccessRule{}).Error
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

	if err := db.Model(&entity.AccessRule{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&policies).Error; err != nil {
		return nil, err
	}

	result.Data = policies
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size
	return result, nil
}

func (a accessPolicyServiceImpl) CheckAccess(ctx context.Context, resourceType utils.ResourceType, resourceId uint, action string) (bool, error) {
	var (
		err   error
		count int64
	)

	userId := ctx.Value("userId")
	//check whether resource is own
	switch resourceType {
	case utils.Group:
		var group entity.NodeGroup
		if err = a.Model(&entity.NodeGroup{}).Where("id = ?", resourceId).Find(&group).Error; err != nil {
			return false, err
		}

		if group.OwnerId == userId {
			return true, nil
		}
	case utils.Policy:
		var policy entity.NodeGroup
		if err = a.Model(&entity.AccessPolicy{}).Where("id = ?", resourceId).Find(&policy).Error; err != nil {
			return false, err
		}

		if policy.OwnerId == userId {
			return true, nil
		}

	case utils.Node:
		var node entity.NodeGroup
		if err = a.Model(&entity.Node{}).Where("id = ?", resourceId).Find(&node).Error; err != nil {
			return false, err
		}

		if node.OwnerId == userId {
			return true, nil
		}

	case utils.Label:
		var label entity.Label
		if err = a.Model(&entity.Label{}).Where("id = ?", resourceId).Find(&label).Error; err != nil {
			return false, err
		}

		if label.OwnerId == userId {
			return true, nil
		}

	case utils.Rule:
		var rule entity.AccessRule
		if err = a.Model(&entity.AccessRule{}).Where("id = ?", resourceId).Find(&rule).Error; err != nil {
			return false, err
		}

		if rule.OwnerId == userId {
			return true, nil
		}

	default:
		return false, nil
	}

	//check whether user has permission
	if err = a.Model(&entity.UserResourceGrantedPermission{}).Where("invitation_id = ? and resource_id = ? and permission_value =  ?", userId, resourceId, action).Count(&count).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
	}

	//check whether user has permission
	if count == 0 {
		return false, errors.New("no permission")
	}

	return true, nil

}

func (a accessPolicyServiceImpl) BatchCheckAccess(ctx context.Context, requests []AccessRequest) ([]AccessResult, error) {
	//TODO implement me
	panic("implement me")
}

func (a accessPolicyServiceImpl) GetAccessLogs(ctx context.Context, filter AccessLogFilter) ([]entity.AccessLog, error) {
	//TODO implement me
	panic("implement me")
}

func (a accessPolicyServiceImpl) ListPermissions(ctx context.Context, params *dto.PermissionParams) (*vo.PageVo, error) {
	sql, wrappers := utils.GenerateSql(params)
	var permissions []entity.Permissions
	var result = new(vo.PageVo)
	db := a.DB
	if sql != "" {
		db = db.Model(&entity.Permissions{}).Where(sql, wrappers)
	}

	if err := db.Model(&entity.Permissions{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := db.Model(&entity.Permissions{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&permissions).Error; err != nil {
		return nil, err
	}

	var vos []vo.PermissionVo
	for _, permission := range permissions {
		vos = append(vos, vo.PermissionVo{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
		})
	}

	result.Data = vos
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size
	return result, nil
}

func (a accessPolicyServiceImpl) QueryPermissions(ctx context.Context, params *dto.PermissionParams) ([]*vo.PermissionVo, error) {
	sql, wrappers := utils.GenerateSql(params)
	var permissions []entity.Permissions
	db := a.DB
	if sql != "" {
		db = db.Model(&entity.Permissions{}).Where(sql, wrappers)
	}

	if err := db.Model(&entity.Permissions{}).Find(&permissions).Error; err != nil {
		return nil, err
	}

	var vos []*vo.PermissionVo
	for _, permission := range permissions {
		vos = append(vos, &vo.PermissionVo{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
		})
	}

	return vos, nil
}

func (a accessPolicyServiceImpl) DeleteUserResourcePermission(ctx context.Context, inviteId, permissionId uint) error {
	var (
		err error
	)

	return a.DB.Transaction(func(tx *gorm.DB) error {
		if err = tx.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ? and permission_id = ?", inviteId, permissionId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
			return err
		}

		return nil
	})

}

func NewAccessPolicyService(db *DatabaseService) AccessPolicyService {
	return &accessPolicyServiceImpl{
		logger:          log.NewLogger(log.Loglevel, "access_policy_service"),
		DatabaseService: db,
	}
}
