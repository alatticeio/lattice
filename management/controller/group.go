package controller

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type GroupController struct {
	logger       *log.Logger
	groupService service.GroupService
}

func NewGroupController(db *service.DatabaseService) *GroupController {
	return &GroupController{
		groupService: service.NewGroupService(db),
		logger:       log.NewLogger(log.Loglevel, "group-policy-controller")}
}

func (g *GroupController) ListGroupPolicies(ctx context.Context, params *dto.GroupPolicyParams) ([]*vo.GroupPolicyVo, error) {
	return g.groupService.ListGroupPolicy(ctx, params)
}

func (g *GroupController) DeleteGroupPolicy(ctx context.Context, groupId uint, policyId uint) error {
	return g.groupService.DeleteGroupPolicy(ctx, groupId, policyId)
}

func (g *GroupController) DeleteGroupNode(ctx context.Context, groupId uint, nodeId uint) error {
	return g.groupService.DeleteGroupNode(ctx, groupId, nodeId)
}

func (p *GroupController) GetNodeGroup(ctx context.Context, id string) (*vo.Group, error) {
	return p.groupService.GetNodeGroup(ctx, id)
}

// CreateGroup NodeGroup module
func (p *GroupController) CreateGroup(ctx context.Context, dto *dto.NodeGroupDto) (*entity.NodeGroup, error) {
	return nil, p.groupService.CreateGroup(ctx, dto)
}

func (p *GroupController) UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	return p.groupService.UpdateGroup(ctx, dto)
}

func (p *GroupController) DeleteGroup(ctx context.Context, id string) error {
	return p.groupService.DeleteGroup(ctx, id)
}

func (p *GroupController) ListGroups(ctx context.Context, params *dto.GroupParams) (*vo.PageVo, error) {
	return p.groupService.ListGroups(ctx, params)
}

func (p *GroupController) QueryGroups(ctx context.Context, params *dto.GroupParams) ([]*vo.Group, error) {
	return p.groupService.QueryGroups(ctx, params)
}
