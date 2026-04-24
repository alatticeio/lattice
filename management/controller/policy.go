package controller

import (
	"context"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/resource"
	"wireflow/management/service"
	"wireflow/management/vo"
)

type PolicyController interface {
	ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error)
	Submit(ctx context.Context, wsID, createdBy, createdByName string, policyDto *dto.PolicyDto) (*models.Policy, error)
	ApplyDirect(ctx context.Context, wsID string, policyDto *dto.PolicyDto) (*vo.PolicyVo, error)
	Apply(ctx context.Context, policyID string) error
	DeletePolicy(ctx context.Context, name string) error
}

type policyController struct {
	policyService service.PolicyService
}

func (p *policyController) ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error) {
	return p.policyService.ListPolicy(ctx, pageParam)
}

func (p *policyController) Submit(ctx context.Context, wsID, createdBy, createdByName string, policyDto *dto.PolicyDto) (*models.Policy, error) {
	return p.policyService.Submit(ctx, wsID, createdBy, createdByName, policyDto)
}

func (p *policyController) ApplyDirect(ctx context.Context, wsID string, policyDto *dto.PolicyDto) (*vo.PolicyVo, error) {
	return p.policyService.ApplyDirect(ctx, wsID, policyDto)
}

func (p *policyController) Apply(ctx context.Context, policyID string) error {
	return p.policyService.Apply(ctx, policyID)
}

func (p *policyController) DeletePolicy(ctx context.Context, name string) error {
	return p.policyService.DeletePolicy(ctx, name)
}

func NewPolicyController(client *resource.Client, st store.Store) PolicyController {
	return &policyController{
		policyService: service.NewPolicyService(client, st),
	}
}
