package controller

import (
	"linkany/control/entity"
	"linkany/control/mapper"
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

func (p *PeerController) List(userId string) ([]*entity.Peer, error) {
	return p.peerMapper.List(userId)
}
