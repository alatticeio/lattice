package service

import (
	"context"
	"errors"
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/grpc/mgt"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
)

// NodeService is an interface for peer mapper
type NodeService interface {
	Register(e *dto.PeerDto) (*entity.Node, error)
	Update(e *dto.PeerDto) (*entity.Node, error)
	Delete(e *dto.PeerDto) error

	// GetByAppId returns a peer by appId, every client has its own appId
	GetByAppId(appId string) (*entity.Node, error)

	GetNetworkMap(appId, userId string) (*entity.NetworkMap, error)

	// List returns a list of peers by userId，when client start up, it will call this method to get all the peers once
	// after that, it will call Watch method to get the latest peers
	List(params *dto.QueryParams) ([]*entity.Node, error)

	// Watch returns a channel that will be used to send the latest peers to the client
	//Watch() (<-chan *entity.Node, error)

	//NodeGroup
	GetNodeGroup(id string) (*entity.NodeGroup, error)
	CreateNodeGroup(group *entity.NodeGroup) error
	UpdateNodeGroup(id string, group *entity.NodeGroup) error
	DeleteNodeGroup(id string) error
	ListNodeGroups() ([]*entity.NodeGroup, error)

	//Group memeber
	AddGroupMember(member *entity.GroupMember) error
	RemoveGroupMember(memberID string) error
	ListGroupMembers(groupID string) ([]*entity.GroupMember, error)
	GetGroupMember(memberID string) (*entity.GroupMember, error)

	//Node Label
	AddNodeTag(ctx context.Context, dto *dto.TagDto) error
	UpdateNodeTag(ctx context.Context, dto *dto.TagDto) error
	RemoveNodeTag(ctx context.Context, tagId uint64) error
	ListNodeTags(ctx context.Context, params *dto.LabelParams) (*dto.PageVo, error)
}

var (
	_ NodeService = (*nodeServiceImpl)(nil)
)

type nodeServiceImpl struct {
	logger *log.Logger
	*DatabaseService
}

func NewNodeService(db *DatabaseService) NodeService {
	return &nodeServiceImpl{DatabaseService: db, logger: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "peermapper"))}
}

func (p *nodeServiceImpl) Register(e *dto.PeerDto) (*entity.Node, error) {
	count := p.GetAddress() + 1
	if count == -1 {
		return nil, errors.New("the address can not be allocated")
	}

	addressIP := fmt.Sprintf("10.0.%d.%d", (count-1)/254, ((count-1)%254)+1)

	peer := &entity.Node{
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

func (p *nodeServiceImpl) Update(e *dto.PeerDto) (*entity.Node, error) {
	var node entity.Node
	if err := p.Where("public_key = ?", e.PublicKey).First(&node).Error; err != nil {
		return nil, err
	}
	node.Status = e.Status

	p.Save(node)

	return &node, nil
}

func (p *nodeServiceImpl) Delete(e *dto.PeerDto) error {
	//TODO implement me
	panic("implement me")
}

func (p *nodeServiceImpl) GetByAppId(appId string) (*entity.Node, error) {
	var peer entity.Node
	if err := p.Where("app_id = ?", appId).Find(&peer).Error; err != nil {
		return nil, err
	}

	return &peer, nil
}

// List params will filter
func (p *nodeServiceImpl) List(params *dto.QueryParams) ([]*entity.Node, error) {
	var peers []*entity.Node

	var sql string
	var wrappers []interface{}

	sql, wrappers = utils.Generate(params)

	p.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := p.Where(sql, wrappers...).Find(&peers).Error; err != nil {
		return nil, err
	}

	return peers, nil
}

// Watch when register or update called, first call Watch
func (p *nodeServiceImpl) Watch(appId string) (<-chan *mgt.ManagementMessage, error) {

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
func (p *nodeServiceImpl) GetNetworkMap(appId, userId string) (*entity.NetworkMap, error) {
	current, err := p.GetByAppId(appId)
	if err != nil {
		return nil, err
	}

	var status = 1
	peers, err := p.List(&dto.QueryParams{
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
func (p *nodeServiceImpl) GetAddress() int64 {
	var count int64
	if err := p.Model(&entity.Node{}).Count(&count).Error; err != nil {
		p.logger.Errorf("err： %s", err.Error())
		return -1
	}
	if count > 253 {
		return -1
	}
	return count
}

//NodeGroup

func (p *nodeServiceImpl) GetNodeGroup(id string) (*entity.NodeGroup, error) {
	//TODO implement me
	panic("implement me")
}

func (p *nodeServiceImpl) CreateNodeGroup(group *entity.NodeGroup) error {
	//TODO implement me
	panic("implement me")
}

func (p *nodeServiceImpl) UpdateNodeGroup(id string, group *entity.NodeGroup) error {
	//TODO implement me
	panic("implement me")
}

func (p *nodeServiceImpl) DeleteNodeGroup(id string) error {
	//TODO implement me
	panic("implement me")
}

func (p *nodeServiceImpl) ListNodeGroups() ([]*entity.NodeGroup, error) {
	//TODO implement me
	panic("implement me")
}

func (p *nodeServiceImpl) AddGroupMember(member *entity.GroupMember) error {
	if err := p.Create(member).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) RemoveGroupMember(memberID string) error {
	if err := p.Where("id = ?", memberID).Delete(&entity.GroupMember{}).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) ListGroupMembers(groupID string) ([]*entity.GroupMember, error) {
	var members []*entity.GroupMember
	if err := p.Where("group_id = ?", groupID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (p *nodeServiceImpl) GetGroupMember(memberID string) (*entity.GroupMember, error) {
	var member entity.GroupMember
	if err := p.Where("id = ?", memberID).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

// Node Tags
func (p *nodeServiceImpl) AddNodeTag(ctx context.Context, dto *dto.TagDto) error {
	return p.Create(&entity.Label{
		Label:     dto.Label,
		CreatedBy: dto.Username,
	}).Error
}

func (p *nodeServiceImpl) UpdateNodeTag(ctx context.Context, dto *dto.TagDto) error {
	var tag entity.Label
	if err := p.Where("id = ?", dto.ID).Find(&tag).Error; err != nil {
		return err
	}

	tag.Label = dto.Label
	tag.UpdatedBy = dto.Username
	p.Save(tag)
	return nil
}

func (p *nodeServiceImpl) RemoveNodeTag(ctx context.Context, tagId uint64) error {
	return p.Where("id = ?", tagId).Delete(&entity.Label{}).Error
}

func (p *nodeServiceImpl) ListNodeTags(ctx context.Context, params *dto.LabelParams) (*dto.PageVo, error) {
	var labels []vo.LabelVo
	result := new(dto.PageVo)
	sql, wrappers := utils.Generate(params)
	db := p.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	if err := db.Model(&entity.Label{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := db.Model(&entity.Label{}).Offset((params.PageNo - 1) * params.PageSize).Limit(params.PageSize).Find(&labels).Error; err != nil {
		return nil, err
	}

	result.Data = labels

	return result, nil
}
