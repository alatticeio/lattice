package controller

import (
	"context"
	"linkany/management/service"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type SharedController struct {
	logger        *log.Logger
	sharedService service.SharedService
}

func (s *SharedController) DeleteSharedLabel(ctx context.Context, inviteId, labelId uint) error {
	return s.sharedService.DeleteSharedLabel(ctx, inviteId, labelId)
}

func (s *SharedController) DeleteSharedNode(ctx context.Context, inviteId, nodeId uint) error {
	return s.sharedService.DeleteSharedNode(ctx, inviteId, nodeId)
}

func (s *SharedController) DeleteSharedPolicy(ctx context.Context, inviteId, policyId uint) error {
	return s.sharedService.DeleteSharedPolicy(ctx, inviteId, policyId)
}

func (s *SharedController) DeleteSharedGroup(ctx context.Context, inviteId, groupId uint) error {
	return s.sharedService.DeleteSharedGroup(ctx, inviteId, groupId)
}

func (s *SharedController) GetSharedLabel(ctx context.Context, id string) (*vo.SharedLabelVo, error) {
	return s.sharedService.GetSharedLabel(ctx, id)
}
