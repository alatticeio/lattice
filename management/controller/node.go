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

func (p *NodeController) GetByAppId(appId string) (*entity.Node, error) {
	return p.nodeService.GetByAppId(appId)
}

func (p *NodeController) List(params *service.QueryParams) ([]*entity.Node, error) {
	return p.nodeService.List(params)
}

func (p *NodeController) Update(dto *dto.PeerDto) (*entity.Node, error) {
	return p.nodeService.Update(dto)
}

func (p *NodeController) GetNetworkMap(appId, userId string) (*entity.NetworkMap, error) {
	return p.nodeService.GetNetworkMap(appId, userId)
}

func (p *NodeController) Registry(peer *dto.PeerDto) (*entity.Node, error) {
	return p.nodeService.Register(peer)
}
