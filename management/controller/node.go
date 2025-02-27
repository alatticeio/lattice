package controller

import (
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/service"
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

// NodeGroup module
func (p *NodeController) CreateGroup(dto *dto.NodeGroupDto) (*entity.NodeGroup, error) {
	return nil, p.nodeService.CreateNodeGroup(&entity.NodeGroup{})
}

func (p *NodeController) UpdateGroup(id string, dto *dto.NodeGroupDto) error {
	return p.nodeService.UpdateNodeGroup(id, &entity.NodeGroup{})
}

func (p *NodeController) DeleteGroup(id string) error {
	return p.nodeService.DeleteNodeGroup(id)
}

func (p *NodeController) ListGroups() ([]*entity.NodeGroup, error) {
	return p.nodeService.ListNodeGroups()
}

// Group member
func (p *NodeController) AddGroupMember(dto *dto.GroupMember) error {
	return p.nodeService.AddGroupMember(&entity.GroupMember{})
}

func (p *NodeController) RemoveGroupMember(memberID string) error {
	return p.nodeService.RemoveGroupMember(memberID)
}

func (p *NodeController) ListGroupMembers(groupID string) ([]*entity.GroupMember, error) {
	return p.nodeService.ListGroupMembers(groupID)
}

func (p *NodeController) GetGroupMember(memberID string) (*entity.GroupMember, error) {
	return p.nodeService.GetGroupMember(memberID)
}
