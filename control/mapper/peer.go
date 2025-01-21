package mapper

import (
	"linkany/control/dto"
	"linkany/control/entity"
)

var (
	_ PeerInterface = (*PeerMapper)(nil)
)

type PeerMapper struct {
	*DatabaseService
}

func (p *PeerMapper) Register(e *dto.PeerDto) (*entity.Peer, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PeerMapper) Update(e *dto.PeerDto) (*entity.Peer, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PeerMapper) Delete(e *dto.PeerDto) error {
	//TODO implement me
	panic("implement me")
}

func (p *PeerMapper) GetByAppId(appId string) (*entity.Peer, error) {
	var peer entity.Peer
	if err := p.Where("app_id = ?", appId).Find(&peer).Error; err != nil {
		return nil, err
	}

	return &peer, nil
}

func (p *PeerMapper) FetchAll() ([]*entity.Peer, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PeerMapper) Watch() (<-chan *entity.Peer, error) {
	//TODO implement me
	panic("implement me")
}
