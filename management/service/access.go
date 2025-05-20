package service

import (
	"context"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/repository"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/linkerrors"
	"linkany/pkg/log"
)

type AccessPolicyService interface {
	// Policy manager
	CreatePolicy(ctx context.Context, policyDto *dto.AccessPolicyDto) error
	UpdatePolicy(ctx context.Context, policyDto *dto.AccessPolicyDto) error
	DeletePolicy(ctx context.Context, policyID uint64) error
	GetPolicy(ctx context.Context, policyID uint64) (*entity.AccessPolicy, error)
	ListGroupPolicies(ctx context.Context, params *dto.AccessPolicyParams) (*vo.PageVo, error)

	//ListPagePolicies list with query
	QueryPolicies(ctx context.Context, params *dto.AccessPolicyParams) ([]*vo.AccessPolicyVo, error)
	DeleteUserResourcePermission(ctx context.Context, inviteId, permissionId uint) error

	// Rule manager
	AddRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error
	GetRule(ctx context.Context, id uint64) (*vo.AccessRuleVo, error)
	UpdateRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error
	DeleteRule(ctx context.Context, ruleID uint64) error
	ListPolicyRules(ctx context.Context, params *dto.AccessPolicyRuleParams) (*vo.PageVo, error)
	QueryPolicyRules(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*vo.AccessRuleVo, error)

	// Access control
	CheckAccess(ctx context.Context, resourceType utils.ResourceType, resourceId uint64, action string) (bool, error)
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
	logger             *log.Logger
	db                 *gorm.DB
	groupRepo          repository.GroupRepository
	policyRepo         repository.PolicyRepository
	ruleRepo           repository.RuleRepository
	policyRuleRepo     repository.PolicyRuleRepository
	permissionRepo     repository.PermissionRepository
	sharedRepo         repository.SharedRepository
	nodeRepo           repository.NodeRepository
	labelRepo          repository.LabelRepository
	userPermissionRepo repository.UserResourcePermissionRepository
}

func NewAccessPolicyService(db *gorm.DB) AccessPolicyService {
	return &accessPolicyServiceImpl{
		logger:             log.NewLogger(log.Loglevel, "access_policy_service"),
		db:                 db,
		groupRepo:          repository.NewGroupRepository(db),
		policyRepo:         repository.NewPolicyRepository(db),
		ruleRepo:           repository.NewRuleRepository(db),
		permissionRepo:     repository.NewPermissionRepository(db),
		policyRuleRepo:     repository.NewPolicyRuleRepository(db),
		sharedRepo:         repository.NewSharedRepository(db),
		nodeRepo:           repository.NewNodeRepository(db),
		labelRepo:          repository.NewLabelRepository(db),
		userPermissionRepo: repository.NewUserPermissionRepository(db),
	}
}

func (a *accessPolicyServiceImpl) CreatePolicy(ctx context.Context, policyDto *dto.AccessPolicyDto) error {
	var (
		count int64
	)
	_, count, _ = a.policyRepo.List(ctx, &dto.AccessPolicyParams{
		Name:    policyDto.Name,
		GroupId: policyDto.GroupID,
	})

	if count > 0 {
		return fmt.Errorf("policy with name %s already exists", policyDto.Name)
	}

	return a.policyRepo.Create(ctx, &entity.AccessPolicy{
		Name:        policyDto.Name,
		GroupID:     policyDto.GroupID,
		Priority:    policyDto.Priority,
		Effect:      policyDto.Effect,
		Description: policyDto.Description,
		Status:      policyDto.Status,
		CreatedBy:   policyDto.CreatedBy,
		UpdatedBy:   policyDto.UpdatedBy,
	})
}

func (a *accessPolicyServiceImpl) UpdatePolicy(ctx context.Context, policyDto *dto.AccessPolicyDto) error {

	policy := &entity.AccessPolicy{
		Name:        policyDto.Name,
		GroupID:     policyDto.GroupID,
		Priority:    policyDto.Priority,
		Effect:      policyDto.Effect,
		Description: policyDto.Description,
		Status:      policyDto.Status,
		CreatedBy:   policyDto.CreatedBy,
		UpdatedBy:   policyDto.UpdatedBy,
	}
	policy.ID = policyDto.ID
	return a.policyRepo.Update(ctx, policy)
}

func (a *accessPolicyServiceImpl) DeletePolicy(ctx context.Context, id uint64) error {
	return a.policyRepo.Delete(ctx, id)
}

func (a *accessPolicyServiceImpl) GetPolicy(ctx context.Context, id uint64) (*entity.AccessPolicy, error) {
	return a.policyRepo.Find(ctx, id)
}

func (a *accessPolicyServiceImpl) ListGroupPolicies(ctx context.Context, params *dto.AccessPolicyParams) (*vo.PageVo, error) {
	var (
		policies []*entity.AccessPolicy
		result   = new(vo.PageVo)
		err      error
		count    int64
	)

	policies, count, err = a.policyRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	var vos []*vo.AccessPolicyVo
	for _, policy := range policies {
		vos = append(vos, &vo.AccessPolicyVo{
			ID:          policy.ID,
			Name:        policy.Name,
			GroupID:     policy.GroupID,
			Priority:    policy.Priority,
			Effect:      policy.Effect,
			Description: policy.Description,
			Status:      policy.Status,
			CreatedBy:   policy.CreatedBy,
			UpdatedBy:   policy.UpdatedBy,
			CreatedAt:   policy.CreatedAt,
			UpdatedAt:   policy.UpdatedAt,
		})
	}

	result.Data = vos
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size
	result.Total = count
	return result, err
}

func (a *accessPolicyServiceImpl) QueryPolicyRules(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*vo.AccessRuleVo, error) {
	var (
		rules []*entity.AccessRule
		err   error
	)

	rules, err = a.policyRuleRepo.Query(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(rules) == 0 {
		return nil, fmt.Errorf("no rules found")
	}

	var vos []*vo.AccessRuleVo
	for _, rule := range rules {
		vos = append(vos, &vo.AccessRuleVo{
			ID:         rule.ID,
			Name:       "",
			RuleType:   rule.RuleType,
			PolicyID:   rule.PolicyId,
			SourceType: rule.SourceType,
			SourceID:   rule.SourceId,
			TargetType: rule.TargetType,
			TargetID:   rule.TargetId,
			Actions:    rule.Actions,
			TimeType:   rule.TimeType,
			Conditions: rule.Conditions,
			CreatedAt:  rule.CreatedAt,
			UpdatedAt:  rule.UpdatedAt,
		})
	}

	return vos, nil
}

func (a *accessPolicyServiceImpl) QueryPolicies(ctx context.Context, params *dto.AccessPolicyParams) ([]*vo.AccessPolicyVo, error) {
	var (
		policies []*entity.AccessPolicy
		err      error
	)

	policies, err = a.policyRepo.Query(ctx, params)
	if err != nil {
		return nil, err
	}

	var vos []*vo.AccessPolicyVo
	for _, policy := range policies {
		vos = append(vos, &vo.AccessPolicyVo{
			ID:          policy.ID,
			Name:        policy.Name,
			GroupID:     policy.GroupID,
			Priority:    policy.Priority,
			Effect:      policy.Effect,
			Description: policy.Description,
			Status:      policy.Status,
			CreatedBy:   policy.CreatedBy,
			UpdatedBy:   policy.UpdatedBy,
			CreatedAt:   policy.CreatedAt,
			UpdatedAt:   policy.UpdatedAt,
		})
	}

	return vos, nil
}

func (a *accessPolicyServiceImpl) AddRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error {
	data, err := json.Marshal(ruleDto.Conditions)
	if err != nil {
		return err
	}
	return a.ruleRepo.Create(ctx, &entity.AccessRule{
		OwnId:      utils.GetUserIdFromCtx(ctx),
		PolicyId:   ruleDto.PolicyID,
		SourceType: ruleDto.SourceType,
		SourceId:   ruleDto.SourceID,
		TargetType: ruleDto.TargetType,
		TargetId:   ruleDto.TargetID,
		Actions:    ruleDto.Actions,
		Conditions: string(data),
	})

}

func (a *accessPolicyServiceImpl) GetRule(ctx context.Context, ruleId uint64) (*vo.AccessRuleVo, error) {
	rule, err := a.ruleRepo.Find(ctx, ruleId)
	if err != nil {
		return nil, err
	}
	return &vo.AccessRuleVo{
		ID:         rule.ID,
		RuleType:   rule.RuleType,
		PolicyID:   rule.PolicyId,
		SourceType: rule.SourceType,
		SourceID:   rule.SourceId,
		TargetType: rule.TargetType,
		TargetID:   rule.TargetId,
		Actions:    rule.Actions,
		TimeType:   rule.TimeType,
		Conditions: rule.Conditions,
		CreatedAt:  rule.CreatedAt,
		UpdatedAt:  rule.UpdatedAt,
	}, err
}

func (a *accessPolicyServiceImpl) UpdateRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error {
	return a.ruleRepo.Update(ctx, ruleDto)
}

func (a *accessPolicyServiceImpl) DeleteRule(ctx context.Context, id uint64) error {
	return a.ruleRepo.Delete(ctx, id)
}

func (a *accessPolicyServiceImpl) ListPolicyRules(ctx context.Context, params *dto.AccessPolicyRuleParams) (*vo.PageVo, error) {
	var (
		err    error
		count  int64
		rules  []*entity.AccessRule
		result = new(vo.PageVo)
	)

	if rules, count, err = a.policyRuleRepo.List(ctx, params); err != nil {
		return nil, err
	}

	var vos []*vo.AccessRuleVo
	for _, rule := range rules {

		ruleVo := &vo.AccessRuleVo{
			ID:         rule.ID,
			Name:       "",
			RuleType:   rule.RuleType,
			PolicyID:   rule.PolicyId,
			SourceType: rule.SourceType,
			SourceID:   rule.SourceId,
			Status:     rule.Status,
			TargetType: rule.TargetType,
			TargetID:   rule.TargetId,
			Actions:    rule.Actions,
			TimeType:   rule.TimeType,
			Conditions: rule.Conditions,
			CreatedAt:  rule.CreatedAt,
			UpdatedAt:  rule.UpdatedAt,
		}

		if rule.SourceNode != nil {
			// node resource vo
			nodeResourceVo := new(vo.NodeResourceVo)

			nodeValues := make(map[string]string, 1)
			nodeValues[fmt.Sprintf("%d", rule.SourceNode.ID)] = rule.SourceNode.Name
			nodeResourceVo.NodeValues = nodeValues

			ruleVo.SourceNodeValues = nodeResourceVo

		}

		if rule.TargetNode != nil {
			// node resource vo
			nodeResourceVo := new(vo.NodeResourceVo)

			nodeValues := make(map[string]string, 1)
			nodeValues[fmt.Sprintf("%d", rule.TargetNode.ID)] = rule.TargetNode.Name
			nodeResourceVo.NodeValues = nodeValues

			ruleVo.TargetNodeValues = nodeResourceVo
		}

		if rule.SourceLabel != nil {
			// node resource vo
			labelResourceVo := new(vo.LabelResourceVo)

			nodeValues := make(map[string]string, 1)
			nodeValues[fmt.Sprintf("%d", rule.SourceLabel.ID)] = rule.SourceLabel.Label
			labelResourceVo.LabelValues = nodeValues

			ruleVo.SourceLabelValues = labelResourceVo
		}

		if rule.TargetLabel != nil {
			// node resource vo
			labelResourceVo := new(vo.LabelResourceVo)

			nodeValues := make(map[string]string, 1)
			nodeValues[fmt.Sprintf("%d", rule.TargetLabel.ID)] = rule.TargetLabel.Label
			labelResourceVo.LabelValues = nodeValues

			ruleVo.TargetLabelValues = labelResourceVo
		}

		vos = append(vos, ruleVo)
	}

	result.Data = vos
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size
	result.Total = count
	return result, nil
}

func (a *accessPolicyServiceImpl) CheckAccess(ctx context.Context, resourceType utils.ResourceType, resourceId uint64, action string) (bool, error) {
	var (
		count int64
		err   error
	)

	userId := utils.GetUserIdFromCtx(ctx)
	//check shared
	switch resourceType {
	case utils.Group:
		// first check own
		_, count, err = a.groupRepo.List(ctx, &dto.GroupParams{
			OwnId: &userId,
		})

		if err != nil || count == 0 {
			return false, err
		} else {
			return true, nil
		}

		groups, count, err := a.sharedRepo.ListGroup(ctx, &dto.SharedGroupParams{
			GroupParams: dto.GroupParams{
				GroupId: resourceId,
			},
		})

		if err != nil || count == 0 {
			return false, err
		}

		if groups[0].OwnerId == userId {
			return true, nil
		}
	case utils.Policy:
		var policies []*entity.SharedPolicy
		// first check own
		_, count, err = a.policyRepo.List(ctx, &dto.AccessPolicyParams{
			OwnId: &userId,
		})

		if err != nil || count == 0 {
			return false, err
		} else {
			return true, nil
		}

		policies, count, err = a.sharedRepo.ListPolicy(ctx, &dto.SharedPolicyParams{})

		if err != nil || count == 0 {
			return false, err
		}

		if policies[0].OwnerId == userId {
			return true, nil
		}

	case utils.Node:
		nodes, count, err := a.sharedRepo.ListNode(ctx, &dto.SharedNodeParams{})

		if err != nil || count == 0 {
			return false, err
		}

		if nodes[0].OwnerId == userId {
			return true, nil
		}
	case utils.Label:
		labels, count, err := a.sharedRepo.ListGroup(ctx, &dto.SharedGroupParams{
			GroupParams: dto.GroupParams{
				GroupId: resourceId,
			},
		})

		if err != nil || count == 0 {
			return false, err
		}

		if labels[0].OwnerId == userId {
			return true, nil
		}
	case utils.Rule:
		// first check own
		userId := utils.GetUserIdFromCtx(ctx)
		_, count, err = a.ruleRepo.List(ctx, &dto.AccessPolicyRuleParams{
			OwnId: &userId,
		})

		if err != nil || count == 0 {
			return false, err
		} else {
			return true, nil
		}

		//policies, count, err = a.sharedRepo.ListPolicy(ctx, &dto.SharedPolicyParams{})
		//
		//if err != nil || count == 0 {
		//	return false, err
		//}
		//
		//if policies[0].OwnerId == userId {
		//	return true, nil
		//}

	default:
		return false, nil
	}

	//check whether user has permission
	//if err = a.Model(&entity.UserResourceGrantedPermission{}).Where("invitation_id = ? and resource_id = ? and permission_value =  ?", userId, resourceId, action).Count(&count).Error; err != nil {
	//	if errors.Is(err, gorm.ErrRecordNotFound) {
	//		return false, nil
	//	}
	//}

	// TODO make real params
	a.userPermissionRepo.List(ctx, &dto.AccessPolicyParams{})

	//check whether user has permission
	if count == 0 {
		return false, linkerrors.ErrNoAccessPermissions
	}

	return true, nil

}

func (a *accessPolicyServiceImpl) BatchCheckAccess(ctx context.Context, requests []AccessRequest) ([]AccessResult, error) {
	//TODO implement me
	panic("implement me")
}

func (a *accessPolicyServiceImpl) GetAccessLogs(ctx context.Context, filter AccessLogFilter) ([]entity.AccessLog, error) {
	//TODO implement me
	panic("implement me")
}

func (a *accessPolicyServiceImpl) ListPermissions(ctx context.Context, params *dto.PermissionParams) (*vo.PageVo, error) {
	var (
		err         error
		count       int64
		permissions []*entity.Permissions
		result      = new(vo.PageVo)
	)
	if permissions, count, err = a.permissionRepo.List(ctx, params); err != nil {
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
	result.Total = count
	return result, nil
}

func (a *accessPolicyServiceImpl) QueryPermissions(ctx context.Context, params *dto.PermissionParams) ([]*vo.PermissionVo, error) {
	var (
		err         error
		permissions []*entity.Permissions
		vos         []*vo.PermissionVo
	)
	if permissions, err = a.permissionRepo.Query(ctx, params); err != nil {
		return nil, err
	}

	for _, permission := range permissions {
		vos = append(vos, &vo.PermissionVo{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
		})
	}

	return vos, nil
}

func (a *accessPolicyServiceImpl) DeleteUserResourcePermission(ctx context.Context, inviteId, permissionId uint) error {
	var (
		err error
	)

	return a.db.Transaction(func(tx *gorm.DB) error {
		if err = tx.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ? and permission_id = ?", inviteId, permissionId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
			return err
		}

		return nil
	})

}
