package controller

import (
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/mapper"
)

type PeerController struct {
	peerMapper mapper.PeerInterface
}

func NewPeerController(peerMapper mapper.PeerInterface) *PeerController {
	return &PeerController{peerMapper: peerMapper}
}

func (p *PeerController) GetByAppId(appId string) (*entity.Peer, error) {
	return p.peerMapper.GetByAppId(appId)
}

func (p *PeerController) List(params *mapper.QueryParams) ([]*entity.Peer, error) {
	return p.peerMapper.List(params)
}

func (p *PeerController) Update(dto *dto.PeerDto) (*entity.Peer, error) {
	return p.peerMapper.Update(dto)
}

func (p *PeerController) GetNetworkMap(appId, userId string) (*entity.NetworkMap, error) {
	return p.peerMapper.GetNetworkMap(appId, userId)
}
