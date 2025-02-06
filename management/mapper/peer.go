package mapper

import (
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/grpc/mgt"
	"reflect"
	"strings"
)

// PeerInterface is an interface for peer mapper
type PeerInterface interface {
	Register(e *dto.PeerDto) (*entity.Peer, error)
	Update(e *dto.PeerDto) (*entity.Peer, error)
	Delete(e *dto.PeerDto) error

	// GetByAppId returns a peer by appId, every client has its own appId
	GetByAppId(appId string) (*entity.Peer, error)

	GetNetworkMap(appId, userId string) (*entity.NetworkMap, error)

	// List returns a list of peers by userId，when client start up, it will call this method to get all the peers once
	// after that, it will call Watch method to get the latest peers
	List(params *QueryParams) ([]*entity.Peer, error)

	// Watch returns a channel that will be used to send the latest peers to the client
	//Watch() (<-chan *entity.Peer, error)
}

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
	var peer entity.Peer
	if err := p.Where("pub_key = ?", e.PubKey).First(&peer).Error; err != nil {
		return nil, err
	}

	peer.Online = e.Online

	p.Save(peer)

	return &peer, nil
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

// List params will filter
func (p *PeerMapper) List(params *QueryParams) ([]*entity.Peer, error) {
	var peers []*entity.Peer

	var sql string
	var wrappers []interface{}

	sql, wrappers = Generate(params)

	if err := p.Where(sql, wrappers...).Find(&peers).Error; err != nil {
		return nil, err
	}

	return peers, nil
}

// Generate will generate dynamic sql
func Generate(params *QueryParams) (string, []interface{}) {
	var sb strings.Builder
	var wrappers []interface{}
	filters := params.Generate()
	for i, filter := range filters {
		if i < len(filters)-1 {
			sb.WriteString(fmt.Sprintf("%s = ? and ", filter.Key))
		} else {
			sb.WriteString(fmt.Sprintf("%s = ?", filter.Key))
		}
		wrappers = append(wrappers, reflect.ValueOf(filter.Value).Elem().Interface())
	}

	return sb.String(), wrappers
}

// Watch when register or update called, first call Watch
func (p *PeerMapper) Watch(appId string) (<-chan *mgt.ManagementMessage, error) {

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

// GetNetworkMap get user's network map
func (p *PeerMapper) GetNetworkMap(appId, userId string) (*entity.NetworkMap, error) {
	current, err := p.GetByAppId(appId)
	if err != nil {
		return nil, err
	}

	var online = 1
	peers, err := p.List(&QueryParams{
		PubKey: &current.PublicKey,
		UserId: &userId,
		Online: &online,
	})

	if err != nil {
		return nil, err
	}

	return &entity.NetworkMap{
		UserId: userId,
		Peer:   current,
		Peers:  peers,
	}, nil
}
