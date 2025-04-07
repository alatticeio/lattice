package controller

import (
	"context"
	"linkany/management/dto"
	"linkany/management/service"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type SharedController struct {
	logger        *log.Logger
	sharedService service.SharedService
}

func NewSharedController(db *service.DatabaseService) *SharedController {
	return &SharedController{
		sharedService: service.NewSharedService(db),
		logger:        log.NewLogger(log.Loglevel, "shared-controller")}
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

func (s *SharedController) AddNodeToGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	return s.sharedService.AddNodeToGroup(ctx, dto)
}

func (s *SharedController) AddPolicyToGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	return s.sharedService.AddPolicyToGroup(ctx, dto)
}

func (s *SharedController) ListGroups(ctx context.Context, params *dto.SharedGroupParams) (*vo.PageVo, error) {
	return s.sharedService.ListGroups(ctx, params)
}
