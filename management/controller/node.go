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

type NodeController struct {
	logger      *log.Logger
	nodeService service.NodeService
}

func NewPeerController(db *gorm.DB) *NodeController {
	return &NodeController{
		nodeService: service.NewNodeService(db),
		logger:      log.NewLogger(log.Loglevel, "node-controller")}
}

// GetByAppId get node by appId
func (p *NodeController) GetByAppId(ctx context.Context, appId string) (*entity.Node, error) {
	return p.nodeService.GetByAppId(ctx, appId)
}

// ListNodes lists nodes by params
func (p *NodeController) ListNodes(ctx context.Context, params *dto.QueryParams) (*vo.PageVo, error) {
	return p.nodeService.ListNodes(ctx, params)
}

// QueryNodes lists nodes by params, not contains page and size, used for querying all nodes
func (p *NodeController) QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*vo.NodeVo, error) {
	return p.nodeService.QueryNodes(ctx, params)
}

func (p *NodeController) Update(ctx context.Context, dto *dto.NodeDto) error {
	return p.nodeService.Update(ctx, dto)
}

// UpdateStatus update node's status
func (p *NodeController) UpdateStatus(ctx context.Context, dto *dto.NodeDto) error {
	return p.nodeService.UpdateStatus(ctx, dto)
}

// GetNetworkMap returns the network map for the given appId and userId
func (p *NodeController) GetNetworkMap(ctx context.Context, appId, userId string) (*vo.NetworkMap, error) {
	return p.nodeService.GetNetworkMap(ctx, appId, userId)
}

// Delete deletes a node by appId
func (p *NodeController) Delete(ctx context.Context, appId string) error {
	return p.nodeService.DeleteNode(ctx, appId)
}

// Registry registers a new node
func (p *NodeController) Registry(ctx context.Context, peer *dto.NodeDto) (*entity.Node, error) {
	return p.nodeService.Register(ctx, peer)
}

// CreateAppId creates a new appId for the node
func (p *NodeController) CreateAppId(ctx context.Context) (*entity.Node, error) {
	return p.nodeService.CreateAppId(ctx)
}

// AddGroupMember Add GroupVo Member
func (p *NodeController) AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
	return p.nodeService.AddGroupMember(ctx, dto)
}

func (p *NodeController) RemoveGroupMember(ctx context.Context, ID uint64) error {
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

func (p *NodeController) DeleteLabel(ctx context.Context, id uint64) error {
	return p.nodeService.DeleteLabel(ctx, id)
}

func (p *NodeController) ListLabel(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error) {
	return p.nodeService.ListLabel(ctx, params)
}

func (p *NodeController) GetLabel(ctx context.Context, id uint64) (*entity.Label, error) {
	return p.nodeService.GetLabel(ctx, id)
}

// GroupVo node
func (p *NodeController) AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error {
	return p.nodeService.AddGroupNode(ctx, dto)
}

func (p *NodeController) RemoveGroupNode(ctx context.Context, ID uint64) error {
	return p.nodeService.RemoveGroupNode(ctx, ID)
}

func (p *NodeController) ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error) {
	return p.nodeService.ListGroupNodes(ctx, params)
}

func (p *NodeController) GetGroupNode(ctx context.Context, ID uint64) (*entity.GroupNode, error) {
	return p.nodeService.GetGroupNode(ctx, ID)
}

// Node Label
func (p *NodeController) AddNodeLabel(ctx context.Context, dto *dto.NodeLabelUpdateReq) error {
	return p.nodeService.AddNodeLabel(ctx, dto)
}

func (p *NodeController) RemoveNodeLabel(ctx context.Context, nodeId, labelId uint64) error {
	return p.nodeService.RemoveNodeLabel(ctx, nodeId, labelId)
}

func (p *NodeController) ListNodeLabels(ctx context.Context, params *dto.NodeLabelParams) (*vo.PageVo, error) {
	return p.nodeService.ListNodeLabels(ctx, params)
}

func (p *NodeController) QueryLabels(ctx context.Context, params *dto.LabelParams) ([]*vo.LabelVo, error) {
	return p.nodeService.QueryLabels(ctx, params)
}

// node apis
func (p *NodeController) ListUserNodes(ctx context.Context, params *dto.ApiCommandParams) ([]vo.NodeVo, error) {
	return p.nodeService.ListUserNodes(ctx, params)
}

func (p *NodeController) AddLabel(ctx context.Context, params *dto.ApiCommandParams) error {
	return p.nodeService.AddLabelToNode(ctx, params)
}

func (p *NodeController) ShowLabel(ctx context.Context, params *dto.ApiCommandParams) ([]vo.NodeLabelVo, error) {
	return p.nodeService.ShowLabel(ctx, params)
}

func (p *NodeController) RemoveLabel(ctx context.Context, params *dto.ApiCommandParams) error {
	return p.nodeService.RemoveLabel(ctx, params)
}
