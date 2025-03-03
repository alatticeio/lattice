package controller

import (
	"context"
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type NodeController struct {
	logger      *log.Logger
	nodeService service.NodeService
}

func NewPeerController(nodeService service.NodeService) *NodeController {
	logger := log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "node-controller"))
	return &NodeController{nodeService: nodeService, logger: logger}
}

// Node module
func (p *NodeController) GetByAppId(appId string) (*entity.Node, error) {
	return p.nodeService.GetByAppId(appId)
}

func (p *NodeController) List(params *dto.QueryParams) ([]*entity.Node, error) {
	return p.nodeService.List(params)
}

func (p *NodeController) Update(dto *dto.PeerDto) (*entity.Node, error) {
	return p.nodeService.Update(dto)
}

func (p *NodeController) GetNetworkMap(appId, userId string) (*entity.NetworkMap, error) {
	return p.nodeService.GetNetworkMap(appId, userId)
}

func (p *NodeController) Delete(dto *dto.PeerDto) error {
	return p.nodeService.Delete(dto)
}

func (p *NodeController) Registry(peer *dto.PeerDto) (*entity.Node, error) {
	return p.nodeService.Register(peer)
}

// CreateGroup NodeGroup module
func (p *NodeController) CreateGroup(ctx context.Context, dto *dto.NodeGroupDto) (*entity.NodeGroup, error) {
	return nil, p.nodeService.CreateNodeGroup(ctx, dto)
}

func (p *NodeController) UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	return p.nodeService.UpdateNodeGroup(ctx, dto)
}

func (p *NodeController) DeleteGroup(ctx context.Context, id string) error {
	return p.nodeService.DeleteNodeGroup(ctx, id)
}

func (p *NodeController) ListGroups(ctx context.Context, params *dto.GroupParams) (*vo.PageVo, error) {
	return p.nodeService.ListNodeGroups(ctx, params)
}

// AddGroupMember Add Group Member
func (p *NodeController) AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
	return p.nodeService.AddGroupMember(ctx, dto)
}

func (p *NodeController) RemoveGroupMember(memberID string) error {
	return p.nodeService.RemoveGroupMember(memberID)
}

func (p *NodeController) ListGroupMembers(ctx context.Context, params *dto.GroupMemberParams) (*vo.PageVo, error) {
	return p.nodeService.ListGroupMembers(ctx, params)
}

func (p *NodeController) GetGroupMember(memberID string) (*entity.GroupMember, error) {
	return p.nodeService.GetGroupMember(memberID)
}

// Node tag
func (p *NodeController) CreateTag(ctx context.Context, dto *dto.TagDto) (*entity.Label, error) {
	return nil, p.nodeService.AddNodeTag(ctx, dto)
}

func (p *NodeController) UpdateTag(ctx context.Context, dto *dto.TagDto) error {
	return p.nodeService.UpdateNodeTag(ctx, dto)
}

func (p *NodeController) DeleteTag(ctx context.Context, tagId uint64) error {
	return p.nodeService.RemoveNodeTag(ctx, tagId)
}

func (p *NodeController) ListTags(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error) {
	return p.nodeService.ListNodeTags(ctx, params)
}
