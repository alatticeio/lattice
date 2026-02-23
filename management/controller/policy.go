package controller

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/resource"
	"wireflow/management/service"
	"wireflow/management/vo"
)

type PolicyController interface {
	ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error)
	CreateOrUpdatePolicy(ctx context.Context, peerDto *dto.PolicyDto) (*vo.PolicyVo, error)
	DeletePolicy(ctx context.Context, name string) error
}

type policyController struct {
	policyService service.PolicyService
}

func (p *policyController) DeletePolicy(ctx context.Context, name string) error {
	return p.policyService.DeletePolicy(ctx, name)
}

func (p *policyController) ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error) {
	return p.policyService.ListPolicy(ctx, pageParam)
}

func (p *policyController) CreateOrUpdatePolicy(ctx context.Context, policyDto *dto.PolicyDto) (*vo.PolicyVo, error) {
	return p.policyService.CreateOrUpdatePolicy(ctx, policyDto)
}

func NewPolicyController(client *resource.Client) PolicyController {
	return &policyController{
		policyService: service.NewPolicyService(client),
	}
}
