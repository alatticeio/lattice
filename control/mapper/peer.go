package mapper

import (
	"linkany/control/dto"
	"linkany/control/entity"
	pb "linkany/control/grpc/peer"
)

var (
	_ PeerInterface = (*PeerMapper)(nil)
)

type PeerMapper struct {
	*DatabaseService
}

func NewPeerMapper(db *DatabaseService) *PeerMapper {
	return &PeerMapper{DatabaseService: db}
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

func (p *PeerMapper) List(userId string) ([]*entity.Peer, error) {
	var peers []*entity.Peer
	if err := p.Where("user_id=?", userId).Find(&peers).Error; err != nil {
		return nil, err
	}

	return peers, nil
}

// Watch, when register or update called, first call Watch
func (p *PeerMapper) Watch(appId string) (<-chan *pb.WatchResponse, error) {

	peer, err := p.GetByAppId(appId)
	if err != nil {
		return nil, err
	}

	if peer != nil {
		// Udpate
	} else {
		// Add
	}

	return nil, nil
}
