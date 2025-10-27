package controller

import (
	"context"
	"gorm.io/gorm"
	"wireflow/management/dto"
	"wireflow/management/service"
	"wireflow/management/vo"
	"wireflow/pkg/log"
)

type SharedController struct {
	logger        *log.Logger
	sharedService service.SharedService
}

func NewSharedController(db *gorm.DB) *SharedController {
	return &SharedController{
		sharedService: service.NewSharedService(db),
		logger:        log.NewLogger(log.Loglevel, "shared-controller")}
}

func (s *SharedController) DeleteSharedLabel(ctx context.Context, inviteId, labelId uint64) error {
	return s.sharedService.DeleteSharedLabel(ctx, inviteId, labelId)
}

func (s *SharedController) DeleteSharedNode(ctx context.Context, inviteId, nodeId uint64) error {
	return s.sharedService.DeleteSharedNode(ctx, inviteId, nodeId)
}

func (s *SharedController) DeleteSharedPolicy(ctx context.Context, inviteId, policyId uint64) error {
	return s.sharedService.DeleteSharedPolicy(ctx, inviteId, policyId)
}

func (s *SharedController) DeleteSharedGroup(ctx context.Context, inviteId, groupId uint64) error {
	return s.sharedService.DeleteSharedGroup(ctx, inviteId, groupId)
}

func (s *SharedController) GetSharedLabel(ctx context.Context, id uint64) (*vo.SharedLabelVo, error) {
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

func (s *SharedController) ListNodes(ctx context.Context, params *dto.SharedNodeParams) (*vo.PageVo, error) {
	return s.sharedService.ListNodes(ctx, params)
}

func (s *SharedController) ListLabels(ctx context.Context, params *dto.SharedLabelParams) (*vo.PageVo, error) {
	return s.sharedService.ListLabels(ctx, params)
}

func (s *SharedController) ListPolicies(ctx context.Context, params *dto.SharedPolicyParams) (*vo.PageVo, error) {
	return s.sharedService.ListPolicies(ctx, params)
}
