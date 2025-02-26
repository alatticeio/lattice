package controller

import (
	"context"
	"fmt"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/pkg/log"
)

type AccessController struct {
	log           *log.Logger
	accessService service.AccessPolicyService
}

func NewAccessController(accessService service.AccessPolicyService) *AccessController {
	return &AccessController{accessService: accessService, log: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s ]", "access-controller"))}
}

// AccessRule module
func (a *AccessController) AddRule(rule *entity.AccessRule) error {
	return a.accessService.AddRule(context.Background(), rule)
}

func (a *AccessController) UpdateRule(rule *entity.AccessRule) error {
	return a.accessService.UpdateRule(context.Background(), rule)
}

func (a *AccessController) DeleteRule(ruleID uint) error {
	return a.accessService.DeleteRule(context.Background(), ruleID)
}

func (a *AccessController) ListPolicyRules(policyID uint) ([]entity.AccessRule, error) {
	return a.accessService.ListPolicyRules(context.Background(), policyID)
}

// AccessControl module
func (a *AccessController) CheckAccess(sourceNodeID, targetNodeID uint, action string) (bool, error) {
	return a.accessService.CheckAccess(context.Background(), sourceNodeID, targetNodeID, action)
}

func (a *AccessController) BatchCheckAccess(requests []service.AccessRequest) ([]service.AccessResult, error) {
	return a.accessService.BatchCheckAccess(context.Background(), requests)
}

func (a *AccessController) AddNodeTag(nodeID uint, tag string) error {
	return a.accessService.AddNodeTag(context.Background(), nodeID, tag)
}

func (a *AccessController) RemoveNodeTag(nodeID uint, tag string) error {
	return a.accessService.RemoveNodeTag(context.Background(), nodeID, tag)
}

func (a *AccessController) GetNodeTags(nodeID uint) ([]string, error) {
	return a.accessService.GetNodeTags(context.Background(), nodeID)
}

func (a *AccessController) GetAccessLogs(filter service.AccessLogFilter) ([]entity.AccessLog, error) {
	return a.accessService.GetAccessLogs(context.Background(), filter)
}

// AccessPolicy module
func (a *AccessController) CreatePolicy(policy *entity.AccessPolicy) error {
	return a.accessService.CreatePolicy(context.Background(), policy)
}

func (a *AccessController) UpdatePolicy(policy *entity.AccessPolicy) error {
	return a.accessService.UpdatePolicy(context.Background(), policy)
}

func (a *AccessController) DeletePolicy(policyID uint) error {
	return a.accessService.DeletePolicy(context.Background(), policyID)
}

func (a *AccessController) ListPolicies() ([]entity.AccessPolicy, error) {
	//return a.accessService.ListPolicyRules(context.Background())
	return nil, nil
}

func (a *AccessController) GetPolicy(policyID uint) (*entity.AccessPolicy, error) {
	return a.accessService.GetPolicy(context.Background(), policyID)
}
