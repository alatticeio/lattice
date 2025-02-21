package mapper

import (
	"errors"
	"fmt"
	"k8s.io/klog/v2"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/grpc/mgt"
	"log"
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
	count := p.GetAddress() + 1
	if count == -1 {
		return nil, errors.New("the address can not be allocated")
	}

	addressIP := fmt.Sprintf("10.0.%d.%d", (count-1)/254, ((count-1)%254)+1)

	peer := &entity.Peer{
		InstanceID:          e.InstanceID,
		UserID:              e.UserID,
		Name:                e.Name,
		Hostname:            e.Hostname,
		AppID:               e.AppID,
		Address:             addressIP,
		Endpoint:            e.Endpoint,
		PersistentKeepalive: e.PersistentKeepalive,
		PublicKey:           e.PublicKey,
		PrivateKey:          e.PrivateKey,
		AllowedIPs:          addressIP + "/32",
		RelayIP:             e.RelayIP,
		TieBreaker:          e.TieBreaker,
		Ufrag:               e.Ufrag,
		Pwd:                 e.Pwd,
		Port:                e.Port,
		Status:              e.Status,
	}
	err := p.Create(peer).Error
	if err != nil {
		return nil, err
	}
	return peer, nil
}

func (p *PeerMapper) Update(e *dto.PeerDto) (*entity.Peer, error) {
	var peer entity.Peer
	if err := p.Where("public_key = ?", e.PublicKey).First(&peer).Error; err != nil {
		return nil, err
	}
	peer.Status = e.Status

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

	klog.Infof("sql: %s, wrappers: %v", sql, wrappers)
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

	var status = 1
	peers, err := p.List(&QueryParams{
		PubKey: &current.PublicKey,
		UserId: &userId,
		Status: &status,
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

// GetAddress get peer address
func (p *PeerMapper) GetAddress() int64 {
	var count int64
	if err := p.Model(&entity.Peer{}).Count(&count).Error; err != nil {
		log.Printf("err： %s", err.Error())
		return -1
	}
	if count > 253 {
		return -1
	}
	return count
}
