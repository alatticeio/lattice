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
	"strings"
)

// NodeService is an interface for peer mapper
type NodeService interface {
	Register(e *dto.NodeDto) (*entity.Node, error)
	Update(e *dto.NodeDto) (*entity.Node, error)
	Delete(e *dto.NodeDto) error

	// GetByAppId returns a peer by appId, every client has its own appId
	GetByAppId(appId, userid string) (*entity.Node, int64, error)

	GetNetworkMap(appId, userId string) (*vo.NetworkMap, error)

	// List returns a list of peers by userId，when client start up, it will call this method to get all the peers once
	// after that, it will call Watch method to get the latest peers
	List(params *dto.QueryParams) ([]*vo.NodeVo, error)

	// Watch returns a channel that will be used to send the latest peers to the client
	//Watch() (<-chan *entity.Node, error)

	//Group memeber
	AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error
	RemoveGroupMember(ctx context.Context, ID string) error
	UpdateGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error
	ListGroupMembers(ctx context.Context, params *dto.GroupMemberParams) (*vo.PageVo, error)

	//Node Label
	AddLabel(ctx context.Context, dto *dto.TagDto) error
	UpdateLabel(ctx context.Context, dto *dto.TagDto) error
	DeleteLabel(ctx context.Context, id string) error
	ListLabel(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error)

	//Group Node
	AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error
	RemoveGroupNode(ctx context.Context, id string) error
	ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error)
	GetGroupNode(ctx context.Context, id string) (*entity.GroupNode, error)

	//Node Label
	AddNodeLabel(ctx context.Context, dto *dto.NodeLabelDto) error
	RemoveNodeLabel(ctx context.Context, id string) error
	ListNodeLabels(ctx context.Context, params *dto.NodeLabelParams) (*vo.PageVo, error)
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

func (p *nodeServiceImpl) Register(e *dto.NodeDto) (*entity.Node, error) {
	count := p.GetAddress() + 1
	if count == -1 {
		return nil, errors.New("the address can not be allocated")
	}

	addressIP := fmt.Sprintf("10.0.%d.%d", (count-1)/254, ((count-1)%254)+1)

	peer := &entity.Node{
		Description:         e.Description,
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

func (p *nodeServiceImpl) Update(e *dto.NodeDto) (*entity.Node, error) {
	var node entity.Node
	if err := p.Where("public_key = ?", e.PublicKey).First(&node).Error; err != nil {
		return nil, err
	}
	node.Status = e.Status

	p.Save(node)

	return &node, nil
}

func (p *nodeServiceImpl) Delete(e *dto.NodeDto) error {
	if err := p.Where("id = ?", e.ID).Delete(&entity.Node{}).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) GetByAppId(appId, userId string) (*entity.Node, int64, error) {
	var (
		peer  entity.Node
		count int64
	)

	if err := p.Model(&entity.Node{}).Where("user_id = ?", userId).Count(&count).Error; err != nil {
		return nil, -1, nil
	}

	if err := p.Where("app_id = ?", appId).Find(&peer).Error; err != nil {
		return nil, -1, err
	}

	return &peer, count, nil
}

// List params will filter
func (p *nodeServiceImpl) List(params *dto.QueryParams) ([]*vo.NodeVo, error) {
	var nodes []*entity.Node

	var sql string
	var wrappers []interface{}

	if params.Keyword != nil {
		sql, wrappers = utils.GenerateSql(params)
	} else {
		sql, wrappers = utils.Generate(params)
	}

	p.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := p.Where(sql, wrappers...).Find(&nodes).Error; err != nil {
		return nil, err
	}

	var vos []*vo.NodeVo
	for _, node := range nodes {
		vos = append(vos, &vo.NodeVo{
			ID:                  node.ID,
			Name:                node.Name,
			Description:         node.Description,
			GroupID:             node.GroupID,
			CreatedBy:           node.CreatedBy,
			UserID:              node.UserID,
			Hostname:            node.Hostname,
			AppID:               node.AppID,
			Address:             node.Address,
			Endpoint:            node.Endpoint,
			PersistentKeepalive: node.PersistentKeepalive,
			PublicKey:           node.PublicKey,
			AllowedIPs:          node.AllowedIPs,
			RelayIP:             node.RelayIP,
			TieBreaker:          node.TieBreaker,
			Ufrag:               node.Ufrag,
			Pwd:                 node.Pwd,
			Port:                node.Port,
			Status:              node.Status,
		})
	}
	return vos, nil
}

// Watch when register or update called, first call Watch
func (p *nodeServiceImpl) Watch(appId string) (<-chan *mgt.ManagementMessage, error) {

	peer, _, err := p.GetByAppId(appId, "")
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
func (p *nodeServiceImpl) GetNetworkMap(appId, userId string) (*vo.NetworkMap, error) {
	current, _, err := p.GetByAppId(appId, "")
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

	return &vo.NetworkMap{
		UserId: userId,
		Peer: &vo.NodeVo{
			ID:                  current.ID,
			Name:                current.Name,
			Description:         current.Description,
			GroupID:             current.GroupID,
			CreatedBy:           current.CreatedBy,
			UserID:              current.UserID,
			Hostname:            current.Hostname,
			AppID:               current.AppID,
			Address:             current.Address,
			Endpoint:            current.Endpoint,
			PersistentKeepalive: current.PersistentKeepalive,
			PublicKey:           current.PublicKey,
			AllowedIPs:          current.AllowedIPs,
			RelayIP:             "",
			TieBreaker:          0,
			Ufrag:               "",
			Pwd:                 "",
			Port:                0,
			Status:              current.Status,
		},
		Peers: peers,
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
	if err := p.Where("id = ?", ID).Delete(&entity.GroupMember{}).Error; err != nil {
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
		Role:      dto.Role,
		Status:    dto.Status,
		UpdatedBy: dto.UpdatedBy,
	}
	if err := p.Model(&entity.GroupMember{}).Where("id = ?", dto.ID).Updates(&member).Error; err != nil {
		return err
	}
	return nil
}

// Node Tags
func (p *nodeServiceImpl) AddLabel(ctx context.Context, dto *dto.TagDto) error {
	label := strings.Split(dto.Label, ":")
	if len(label) != 2 || len(label[0]) == 0 || len(label[1]) == 0 {
		return errors.New("invalid label")
	}
	return p.Create(&entity.Label{
		Label:     dto.Label,
		OwnerId:   dto.OwnerId,
		CreatedBy: dto.CreatedBy,
	}).Error
}

func (p *nodeServiceImpl) UpdateLabel(ctx context.Context, dto *dto.TagDto) error {
	var tag entity.Label
	if err := p.Where("id = ?", dto.ID).Find(&tag).Error; err != nil {
		return err
	}

	tag.Label = dto.Label
	tag.UpdatedBy = dto.UpdatedBy
	p.Save(tag)
	return nil
}

func (p *nodeServiceImpl) DeleteLabel(ctx context.Context, id string) error {
	return p.Where("id = ?", id).Delete(&entity.Label{}).Error
}

func (p *nodeServiceImpl) ListLabel(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error) {
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
	}
	if err := p.Create(groupNode).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) RemoveGroupNode(ctx context.Context, ID string) error {
	if err := p.Where("id = ?", ID).Delete(&entity.GroupNode{}).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error) {
	var groupNodes []*entity.GroupNode

	result := new(vo.PageVo)
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

// Node Label
func (p *nodeServiceImpl) AddNodeLabel(ctx context.Context, dto *dto.NodeLabelDto) error {
	var count int64
	if err := p.Model(&entity.Label{}).Where("id = ? and label = ?", dto.LabelID, dto.LabelName).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("invalid label")
	}
	if err := p.Model(&entity.NodeLabel{}).Where("label_id = ? and node_id = ?", dto.LabelID, dto.NodeID).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	nodeLabel := &entity.NodeLabel{
		LabelId:   dto.LabelID,
		LabelName: dto.LabelName,
		NodeId:    dto.NodeID,
		CreatedBy: dto.CreatedBy,
	}
	if err := p.Create(nodeLabel).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) RemoveNodeLabel(ctx context.Context, ID string) error {
	if err := p.Where("id = ?", ID).Delete(&entity.NodeLabel{}).Error; err != nil {
		return err
	}
	return nil
}

func (p *nodeServiceImpl) ListNodeLabels(ctx context.Context, params *dto.NodeLabelParams) (*vo.PageVo, error) {
	var nodeLabels []*entity.NodeLabel

	result := new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := p.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	if err := db.Model(&entity.NodeLabel{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	p.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := db.Model(&entity.NodeLabel{}).Order("created_at DESC").Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&nodeLabels).Error; err != nil {
		return nil, err
	}
	result.Data = nodeLabels

	return result, nil
}
