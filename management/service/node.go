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
	"time"
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

	//Group
	GetNodeGroup(ctx context.Context, id string) (*vo.NodeGroupVo, error)
	CreateGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	DeleteGroup(ctx context.Context, id string) error
	ListGroups(ctx context.Context, params *dto.GroupParams) (*vo.PageVo, error)

	//Group memeber
	AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error
	RemoveGroupMember(ctx context.Context, ID string) error
	UpdateGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error
	ListGroupMembers(ctx context.Context, params *dto.GroupMemberParams) (*vo.PageVo, error)

	//Node Label
	AddNodeTag(ctx context.Context, dto *dto.TagDto) error
	UpdateNodeTag(ctx context.Context, dto *dto.TagDto) error
	RemoveNodeTag(ctx context.Context, tagId uint64) error
	ListNodeTags(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error)

	//Group Node
	AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error
	RemoveGroupNode(ctx context.Context, id string) error
	ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error)
	GetGroupNode(ctx context.Context, id string) (*entity.GroupNode, error)
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
	if err := p.Where("id = ?", e.ID).Delete(&entity.Node{}).Error; err != nil {
		return err
	}
	return nil
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

// NodeGroup
func (p *nodeServiceImpl) GetNodeGroup(ctx context.Context, nodeId string) (*vo.NodeGroupVo, error) {
	var (
		group entity.NodeGroup
		err   error
	)

	if err = p.Where("id = ?", nodeId).First(&group).Error; err != nil {
		return nil, err
	}

	return &vo.NodeGroupVo{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		CreatedAt:   group.CreatedAt,
		DeletedAt:   group.DeletedAt,
		UpdatedAt:   group.UpdatedAt,
		CreatedBy:   group.CreatedBy,
		UpdatedBy:   group.UpdatedBy,
	}, nil
}

func (p *nodeServiceImpl) CreateGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	group := &entity.NodeGroup{
		Name:        dto.Name,
		Description: dto.Description,
		IsPublic:    dto.IsPublic,
		CreatedBy:   dto.CreatedBy,
		UpdatedBy:   dto.CreatedBy,
	}
	var (
		user  *entity.User
		count int64
	)

	if err := p.Model(&entity.NodeGroup{}).Where("name = ? and created_by = ?", group.Name, group.CreatedBy).Count(&count).Error; err != nil {
		return err
	}

	if count != 0 {
		return errors.New("this group already exists")
	}

	if group.CreatedBy != "" {
		if err := p.Where("username = ?", group.CreatedBy).First(&user).Error; err != nil {
			return err
		}
		group.OwnerID = user.ID
	}
	if err := p.Create(group).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	group := &entity.NodeGroup{
		Name:        dto.Name,
		Description: dto.Description,
		IsPublic:    dto.IsPublic,
		CreatedBy:   dto.CreatedBy,
		UpdatedBy:   dto.UpdatedBy,
	}
	var (
		groupOld *entity.NodeGroup
		count    int64
	)

	if err := p.Model(&entity.NodeGroup{}).Where("id = ? and created_by = ?", group.ID, group.CreatedBy).First(&groupOld).Error; err != nil {
		return err
	}

	if groupOld.Name != group.Name {
		if err := p.Model(&entity.NodeGroup{}).Where("name = ? and created_by = ?", group.Name, group.CreatedBy).Count(&count).Error; err != nil {
			return err
		}
	}
	if count != 0 {
		return errors.New("this group already exists")
	}

	if err := p.Model(&entity.NodeGroup{}).Where("id = ? and created_by = ?", group.ID, group.CreatedBy).Updates(group).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) DeleteGroup(ctx context.Context, id string) error {
	if err := p.Where("id = ?", id).Unscoped().Delete(&entity.NodeGroup{}).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) ListGroups(ctx context.Context, params *dto.GroupParams) (*vo.PageVo, error) {
	var nodeGroups []entity.NodeGroup

	result := new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := p.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	if err := db.Model(&entity.NodeGroup{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	p.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := db.Model(&entity.NodeGroup{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&nodeGroups).Error; err != nil {
		return nil, err
	}

	var nodeVos []vo.NodeGroupVo
	for _, group := range nodeGroups {
		nodeVos = append(nodeVos, vo.NodeGroupVo{
			ID:          group.ID,
			Name:        group.Name,
			Description: group.Description,
			CreatedAt:   group.CreatedAt,
			DeletedAt:   group.DeletedAt,
			UpdatedAt:   group.UpdatedAt,
			CreatedBy:   group.CreatedBy,
			UpdatedBy:   group.UpdatedBy,
		})
	}

	result.Data = nodeVos
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size

	return result, nil
}

// Group Members
func (p *nodeServiceImpl) AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
	if dto.Role != "admin" && dto.Role != "owner" && dto.Role != "member" {
		return errors.New("invalid role")
	}
	if dto.Status != "pending" && dto.Status != "accepted" && dto.Status != "rejected" {
		return errors.New("invalid status")
	}
	member := &entity.GroupMember{
		GroupID:   dto.GroupID,
		GroupName: dto.GroupName,
		UserID:    dto.UserID,
		Username:  dto.Username,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: dto.CreatedBy,
		Role:      dto.Role,
		Status:    dto.Status,
	}
	if err := p.Create(member).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) RemoveGroupMember(ctx context.Context, ID string) error {
	if err := p.Where("id = ?", ID).Unscoped().Delete(&entity.GroupMember{}).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) ListGroupMembers(ctx context.Context, params *dto.GroupMemberParams) (*vo.PageVo, error) {
	var groupMember []*entity.GroupMember

	result := new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := p.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	if err := db.Model(&entity.GroupMember{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	p.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := db.Model(&entity.GroupMember{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&groupMember).Error; err != nil {
		return nil, err
	}
	result.Data = groupMember

	return result, nil
}

func (p *nodeServiceImpl) UpdateGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
	member := entity.GroupMember{
		GroupID:   dto.GroupID,
		Username:  dto.Username,
		Role:      dto.Role,
		Status:    dto.Status,
		UpdatedAt: time.Now(),
		UpdatedBy: dto.UpdatedBy,
	}
	if err := p.Model(&entity.GroupMember{}).Where("group_id = ? and username = ?", member.GroupID, member.Username).Updates(&member).Error; err != nil {
		return err
	}
	return nil
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

func (p *nodeServiceImpl) ListNodeTags(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error) {
	var labels []vo.LabelVo
	result := new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := p.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	if err := db.Model(&entity.Label{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := db.Model(&entity.Label{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&labels).Error; err != nil {
		return nil, err
	}

	result.Data = labels

	return result, nil
}

// Group Node
func (p *nodeServiceImpl) AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error {
	groupNode := &entity.GroupNode{
		GroupID:   dto.GroupID,
		NodeID:    dto.NodeID,
		GroupName: dto.GroupName,
		CreatedBy: dto.CreatedBy,
		CreatedAt: time.Now(),
	}
	if err := p.Create(groupNode).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) RemoveGroupNode(ctx context.Context, ID string) error {
	if err := p.Where("id = ?", ID).Unscoped().Delete(&entity.GroupNode{}).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error) {
	var groupNodes []*entity.GroupNode

	result := new(vo.PageVo)
	fmt.Println(params)
	sql, wrappers := utils.Generate(params)
	db := p.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	if err := db.Model(&entity.GroupNode{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	p.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := db.Model(&entity.GroupNode{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&groupNodes).Error; err != nil {
		return nil, err
	}
	result.Data = groupNodes

	return result, nil
}

func (p *nodeServiceImpl) GetGroupNode(ctx context.Context, ID string) (*entity.GroupNode, error) {
	var groupNode entity.GroupNode
	if err := p.Where("id = ?", ID).First(&groupNode).Error; err != nil {
		return nil, err
	}
	return &groupNode, nil
}
