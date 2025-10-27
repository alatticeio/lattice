package controller

import (
	"context"
	"gorm.io/gorm"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/management/service"
	"wireflow/management/utils"
	"wireflow/management/vo"
	"wireflow/pkg/log"
)

type AccessController struct {
	logger        *log.Logger
	accessService service.AccessPolicyService
}

func NewAccessController(db *gorm.DB) *AccessController {
	return &AccessController{accessService: service.NewAccessPolicyService(db),
		logger: log.NewLogger(log.Loglevel, "access-controller")}
}

// AccessRule module
func (a *AccessController) AddRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error {
	return a.accessService.AddRule(ctx, ruleDto)
}

func (a *AccessController) GetRule(ctx context.Context, id uint64) (*vo.AccessRuleVo, error) {
	return a.accessService.GetRule(ctx, id)
}

func (a *AccessController) UpdateRule(ctx context.Context, ruleDto *dto.AccessRuleDto) error {
	return a.accessService.UpdateRule(ctx, ruleDto)
}

func (a *AccessController) DeleteRule(ruleID uint64) error {
	return a.accessService.DeleteRule(context.Background(), ruleID)
}

func (a *AccessController) ListPolicyRules(ctx context.Context, params *dto.AccessPolicyRuleParams) (*vo.PageVo, error) {
	return a.accessService.ListPolicyRules(ctx, params)
}

func (a *AccessController) QueryPolicyRules(ctx context.Context, params *dto.AccessPolicyRuleParams) ([]*vo.AccessRuleVo, error) {
	return a.accessService.QueryPolicyRules(ctx, params)
}

// AccessControl module
func (a *AccessController) CheckAccess(ctx context.Context, resourceType utils.ResourceType, resourceId uint64, action string) (bool, error) {
	return a.accessService.CheckAccess(ctx, resourceType, resourceId, action)
}

func (a *AccessController) BatchCheckAccess(requests []service.AccessRequest) ([]service.AccessResult, error) {
	return a.accessService.BatchCheckAccess(context.Background(), requests)
}

func (a *AccessController) GetAccessLogs(filter service.AccessLogFilter) ([]entity.AccessLog, error) {
	return a.accessService.GetAccessLogs(context.Background(), filter)
}

// AccessPolicy module
func (a *AccessController) CreatePolicy(ctx context.Context, dto *dto.AccessPolicyDto) error {
	return a.accessService.CreatePolicy(ctx, dto)
}

func (a *AccessController) UpdatePolicy(ctx context.Context, policyDto *dto.AccessPolicyDto) error {
	return a.accessService.UpdatePolicy(ctx, policyDto)
}

func (a *AccessController) DeletePolicy(ctx context.Context, policyID uint64) error {
	return a.accessService.DeletePolicy(ctx, policyID)
}

func (a *AccessController) ListPagePolicies(ctx context.Context, params *dto.AccessPolicyParams) (*vo.PageVo, error) {
	return a.accessService.ListGroupPolicies(ctx, params)
}

func (a *AccessController) ListPolicies(ctx context.Context, params *dto.AccessPolicyParams) ([]*vo.AccessPolicyVo, error) {
	return a.accessService.QueryPolicies(ctx, params)
}

func (a *AccessController) GetPolicy(ctx context.Context, policyID uint64) (*entity.AccessPolicy, error) {
	return a.accessService.GetPolicy(ctx, policyID)
}

func (a *AccessController) QueryPermissions(ctx context.Context, params *dto.PermissionParams) ([]*vo.PermissionVo, error) {
	return a.accessService.QueryPermissions(ctx, params)
}

func (a *AccessController) DeleteUserResourcePermission(ctx context.Context, inviteId, permissionId uint) error {
	return a.accessService.DeleteUserResourcePermission(ctx, inviteId, permissionId)
}

func (a *AccessController) ListUserPolicies(ctx context.Context, params *dto.ApiCommandParams) ([]vo.AccessPolicyVo, error) {
	return a.accessService.ListUserPolicies(ctx, params)
}
