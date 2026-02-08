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
	UpdatePolicy(ctx context.Context, peerDto *dto.PeerDto) (*vo.PolicyVo, error)
}

type policyController struct {
	policyService service.PolicyService
}

func (p *policyController) ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error) {
	return p.policyService.ListPolicy(ctx, pageParam)
}

func (p *policyController) UpdatePolicy(ctx context.Context, policyDto *dto.PeerDto) (*vo.PolicyVo, error) {
	return p.policyService.UpdatePolicy(ctx, policyDto)
}

func NewPolicyController(client *resource.Client) PolicyController {
	return &policyController{
		policyService: service.NewPolicyService(client),
	}
}
