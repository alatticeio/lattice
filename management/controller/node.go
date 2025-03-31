package controller

import (
	"context"
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

func NewPeerController(db *service.DatabaseService) *NodeController {
	return &NodeController{
		nodeService: service.NewNodeService(db),
		logger:      log.NewLogger(log.Loglevel, "node-controller")}
}

// Node module
func (p *NodeController) GetByAppId(appId, userId string) (*entity.Node, int64, error) {
	return p.nodeService.GetByAppId(appId, userId)
}

func (p *NodeController) ListNodes(params *dto.QueryParams) (*vo.PageVo, error) {
	return p.nodeService.ListNodes(params)
}

func (p *NodeController) QueryNodes(params *dto.QueryParams) ([]*vo.NodeVo, error) {
	return p.nodeService.QueryNodes(params)
}

func (p *NodeController) Update(dto *dto.NodeDto) (*entity.Node, error) {
	return p.nodeService.Update(dto)
}

func (p *NodeController) GetNetworkMap(appId, userId string) (*vo.NetworkMap, error) {
	return p.nodeService.GetNetworkMap(appId, userId)
}

func (p *NodeController) Delete(ctx context.Context, appId string) error {
	return p.nodeService.DeleteNode(ctx, appId)
}

func (p *NodeController) Registry(peer *dto.NodeDto) (*entity.Node, error) {
	return p.nodeService.Register(peer)
}

func (p *NodeController) CreateAppId(ctx context.Context) (*entity.Node, error) {
	return p.nodeService.CreateAppId(ctx)
}

// AddGroupMember Add Group Member
func (p *NodeController) AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
	return p.nodeService.AddGroupMember(ctx, dto)
}

func (p *NodeController) RemoveGroupMember(ctx context.Context, ID string) error {
	return p.nodeService.RemoveGroupMember(ctx, ID)
}

func (p *NodeController) UpdateGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
	return p.nodeService.UpdateGroupMember(ctx, dto)
}

func (p *NodeController) ListGroupMembers(ctx context.Context, params *dto.GroupMemberParams) (*vo.PageVo, error) {
	return p.nodeService.ListGroupMembers(ctx, params)
}

// Node tag
func (p *NodeController) CreateLabel(ctx context.Context, dto *dto.TagDto) (*entity.Label, error) {
	return nil, p.nodeService.AddLabel(ctx, dto)
}

func (p *NodeController) UpdateLabel(ctx context.Context, dto *dto.TagDto) error {
	return p.nodeService.UpdateLabel(ctx, dto)
}

func (p *NodeController) DeleteLabel(ctx context.Context, id string) error {
	return p.nodeService.DeleteLabel(ctx, id)
}

func (p *NodeController) ListLabel(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error) {
	return p.nodeService.ListLabel(ctx, params)
}

func (p *NodeController) GetLabel(ctx context.Context, id string) (*entity.Label, error) {
	return p.nodeService.GetLabel(ctx, id)
}

// Group node
func (p *NodeController) AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error {
	return p.nodeService.AddGroupNode(ctx, dto)
}

func (p *NodeController) RemoveGroupNode(ctx context.Context, ID string) error {
	return p.nodeService.RemoveGroupNode(ctx, ID)
}

func (p *NodeController) ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error) {
	return p.nodeService.ListGroupNodes(ctx, params)
}

func (p *NodeController) GetGroupNode(ctx context.Context, ID string) (*entity.GroupNode, error) {
	return p.nodeService.GetGroupNode(ctx, ID)
}

// Node Label
func (p *NodeController) AddNodeLabel(ctx context.Context, dto *dto.NodeLabelUpdateReq) error {
	return p.nodeService.AddNodeLabel(ctx, dto)
}

func (p *NodeController) RemoveNodeLabel(ctx context.Context, nodeId, labelId string) error {
	return p.nodeService.RemoveNodeLabel(ctx, nodeId, labelId)
}

func (p *NodeController) ListNodeLabels(ctx context.Context, params *dto.NodeLabelParams) (*vo.PageVo, error) {
	return p.nodeService.ListNodeLabels(ctx, params)
}

func (p *NodeController) QueryLabels(ctx context.Context, params *dto.NodeLabelParams) ([]*vo.LabelVo, error) {
	return p.nodeService.QueryLabels(ctx, params)
}
