package controller

import (
	"context"
	"gorm.io/gorm"
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

func NewGroupController(db *gorm.DB) *GroupController {
	return &GroupController{
		groupService: service.NewGroupService(db),
		logger:       log.NewLogger(log.Loglevel, "group-policy-controller")}
}

func (g *GroupController) ListGroupPolicies(ctx context.Context, params *dto.GroupPolicyParams) ([]*vo.GroupPolicyVo, error) {
	return g.groupService.ListGroupPolicy(ctx, params)
}

func (g *GroupController) DeleteGroupPolicy(ctx context.Context, groupId, policyId uint64) error {
	return g.groupService.DeleteGroupPolicy(ctx, groupId, policyId)
}

func (g *GroupController) DeleteGroupNode(ctx context.Context, groupId, nodeId uint64) error {
	return g.groupService.DeleteGroupNode(ctx, groupId, nodeId)
}

func (p *GroupController) GetNodeGroup(ctx context.Context, id uint64) (*vo.GroupVo, error) {
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

func (p *GroupController) QueryGroups(ctx context.Context, params *dto.GroupParams) ([]*vo.GroupVo, error) {
	return p.groupService.QueryGroups(ctx, params)
}

// api group
func (p *GroupController) JoinGroup(ctx context.Context, params *dto.ApiCommandParams) error {
	return p.groupService.JoinGroup(ctx, params)
}

func (p *GroupController) LeaveGroup(ctx context.Context, params *dto.ApiCommandParams) error {
	return p.groupService.LeaveGroup(ctx, params)
}

func (p *GroupController) RemoveGroup(ctx context.Context, params *dto.ApiCommandParams) error {
	return p.groupService.RemoveGroup(ctx, params)
}

func (p *GroupController) AddGroup(ctx context.Context, params *dto.ApiCommandParams) error {
	return p.groupService.AddGroup(ctx, params)
}
