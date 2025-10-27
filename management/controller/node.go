package controller

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/management/service"
	"wireflow/management/vo"
	"wireflow/pkg/log"

	"gorm.io/gorm"
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
func (n *NodeController) GetByAppId(ctx context.Context, appId string) (*entity.Node, error) {
	return n.nodeService.GetByAppId(ctx, appId)
}

// ListNodes lists nodes by params
func (n *NodeController) ListNodes(ctx context.Context, params *dto.QueryParams) (*vo.PageVo, error) {
	return n.nodeService.ListNodes(ctx, params)
}

// QueryNodes lists nodes by params, not contains page and size, used for querying all nodes
func (n *NodeController) QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*vo.NodeVo, error) {
	return n.nodeService.QueryNodes(ctx, params)
}

func (n *NodeController) Update(ctx context.Context, dto *dto.NodeDto) error {
	return n.nodeService.Update(ctx, dto)
}

// UpdateStatus update node's status
func (n *NodeController) UpdateStatus(ctx context.Context, dto *dto.NodeDto) error {
	return n.nodeService.UpdateStatus(ctx, dto)
}

// GetNetworkMap returns the network map for the given appId and userId
func (n *NodeController) GetNetworkMap(ctx context.Context, appId, userId string) (*vo.NetworkMap, error) {
	return n.nodeService.GetNetworkMap(ctx, appId, userId)
}

// Delete deletes a node by appId
func (n *NodeController) Delete(ctx context.Context, appId string) error {
	return n.nodeService.DeleteNode(ctx, appId)
}

// Registry registers a new node
func (n *NodeController) Registry(ctx context.Context, peer *dto.NodeDto) (*entity.Node, error) {
	return n.nodeService.Register(ctx, peer)
}

// CreateAppId creates a new appId for the node
func (n *NodeController) CreateAppId(ctx context.Context) (*entity.Node, error) {
	return n.nodeService.CreateAppId(ctx)
}

// AddGroupMember Add GroupVo Member
func (n *NodeController) AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
	return n.nodeService.AddGroupMember(ctx, dto)
}

func (n *NodeController) RemoveGroupMember(ctx context.Context, ID uint64) error {
	return n.nodeService.RemoveGroupMember(ctx, ID)
}

func (n *NodeController) UpdateGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
	return n.nodeService.UpdateGroupMember(ctx, dto)
}

func (n *NodeController) ListGroupMembers(ctx context.Context, params *dto.GroupMemberParams) (*vo.PageVo, error) {
	return n.nodeService.ListGroupMembers(ctx, params)
}

// Node tag
func (n *NodeController) CreateLabel(ctx context.Context, dto *dto.TagDto) (*entity.Label, error) {
	return nil, n.nodeService.AddLabel(ctx, dto)
}

func (n *NodeController) UpdateLabel(ctx context.Context, dto *dto.TagDto) error {
	return n.nodeService.UpdateLabel(ctx, dto)
}

func (n *NodeController) DeleteLabel(ctx context.Context, id uint64) error {
	return n.nodeService.DeleteLabel(ctx, id)
}

func (n *NodeController) ListLabel(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error) {
	return n.nodeService.ListLabel(ctx, params)
}

func (n *NodeController) GetLabel(ctx context.Context, id uint64) (*entity.Label, error) {
	return n.nodeService.GetLabel(ctx, id)
}

// GroupVo node
func (n *NodeController) AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error {
	return n.nodeService.AddGroupNode(ctx, dto)
}

func (n *NodeController) RemoveGroupNode(ctx context.Context, ID uint64) error {
	return n.nodeService.RemoveGroupNode(ctx, ID)
}

func (n *NodeController) ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error) {
	return n.nodeService.ListGroupNodes(ctx, params)
}

func (n *NodeController) GetGroupNode(ctx context.Context, ID uint64) (*entity.GroupNode, error) {
	return n.nodeService.GetGroupNode(ctx, ID)
}

// Node Label
func (n *NodeController) AddNodeLabel(ctx context.Context, dto *dto.NodeLabelUpdateReq) error {
	return n.nodeService.AddNodeLabel(ctx, dto)
}

func (n *NodeController) RemoveNodeLabel(ctx context.Context, nodeId, labelId uint64) error {
	return n.nodeService.RemoveNodeLabel(ctx, nodeId, labelId)
}

func (n *NodeController) ListNodeLabels(ctx context.Context, params *dto.NodeLabelParams) (*vo.PageVo, error) {
	return n.nodeService.ListNodeLabels(ctx, params)
}

func (n *NodeController) QueryLabels(ctx context.Context, params *dto.LabelParams) ([]*vo.LabelVo, error) {
	return n.nodeService.QueryLabels(ctx, params)
}

// node apis
func (n *NodeController) ListUserNodes(ctx context.Context, params *dto.ApiCommandParams) ([]vo.NodeVo, error) {
	return n.nodeService.ListUserNodes(ctx, params)
}

func (n *NodeController) AddLabel(ctx context.Context, params *dto.ApiCommandParams) error {
	return n.nodeService.AddLabelToNode(ctx, params)
}

func (n *NodeController) ShowLabel(ctx context.Context, params *dto.ApiCommandParams) ([]vo.NodeLabelVo, error) {
	return n.nodeService.ShowLabel(ctx, params)
}

func (n *NodeController) RemoveLabel(ctx context.Context, params *dto.ApiCommandParams) error {
	return n.nodeService.RemoveLabel(ctx, params)
}
