package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"wireflow/internal/core/domain"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/management/repository"
	"wireflow/management/vo"
	"wireflow/pkg/log"
	utils2 "wireflow/pkg/utils"

	"gorm.io/gorm"
)

// NodeService is an interface for peer mapper
type NodeService interface {
	Register(ctx context.Context, e *dto.NodeDto) (*entity.Node, error)
	CreateAppId(ctx context.Context) (*entity.Node, error)
	Update(ctx context.Context, e *dto.NodeDto) error
	UpdateStatus(ctx context.Context, nodeDto *dto.NodeDto) error
	DeleteNode(ctx context.Context, appId string) error

	// GetByAppId returns a peer by appId, every client has its own appId
	GetByAppId(ctx context.Context, appId string) (*entity.Node, error)

	GetById(ctx context.Context, nodeId uint64) (*entity.Node, error)

	GetNetworkMap(ctx context.Context, appId, userId string) (*vo.NetworkMap, error)

	// GetNetMap returns a list of peers by userId，when client start up, it will call this method to get all the peers once
	// after that, it will call Watch method to get the latest peers
	ListNodes(ctx context.Context, params *dto.QueryParams) (*vo.PageVo, error)

	QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*vo.NodeVo, error)

	//GroupVo memeber
	AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error
	RemoveGroupMember(ctx context.Context, id uint64) error
	UpdateGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error
	ListGroupMembers(ctx context.Context, params *dto.GroupMemberParams) (*vo.PageVo, error)

	//Peer Label
	AddLabel(ctx context.Context, dto *dto.TagDto) error
	UpdateLabel(ctx context.Context, dto *dto.TagDto) error
	DeleteLabel(ctx context.Context, id uint64) error
	ListLabel(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error)
	QueryLabels(ctx context.Context, params *dto.LabelParams) ([]*vo.LabelVo, error)
	GetLabel(ctx context.Context, id uint64) (*entity.Label, error)

	//GroupVo Peer
	AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error
	RemoveGroupNode(ctx context.Context, id uint64) error
	ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error)
	GetGroupNode(ctx context.Context, id uint64) (*entity.GroupNode, error)

	//Peer Label
	AddNodeLabel(ctx context.Context, dto *dto.NodeLabelUpdateReq) error
	RemoveNodeLabel(ctx context.Context, nodeId, labelId uint64) error
	ListNodeLabels(ctx context.Context, params *dto.NodeLabelParams) (*vo.PageVo, error)

	// nodeapis
	ListUserNodes(ctx context.Context, params *dto.ApiCommandParams) ([]vo.NodeVo, error)
	AddLabelToNode(ctx context.Context, dto *dto.ApiCommandParams) error
	RemoveLabel(ctx context.Context, dto *dto.ApiCommandParams) error
	ShowLabel(ctx context.Context, params *dto.ApiCommandParams) ([]vo.NodeLabelVo, error)
}

var (
	_ NodeService = (*nodeServiceImpl)(nil)
)

type nodeServiceImpl struct {
	db              *gorm.DB
	logger          *log.Logger
	nodeRepo        repository.NodeRepository
	groupMemberRepo repository.GroupMemberRepository
	groupNodeRepo   repository.GroupNodeRepository
	groupRepo       repository.GroupRepository
	labelRepo       repository.LabelRepository
	nodeLabelRepo   repository.NodeLabelRepository
	baseRepo        repository.BaseRepository[entity.NodeLabel]
}

func NewNodeService(db *gorm.DB) NodeService {
	return &nodeServiceImpl{
		db:              db,
		nodeRepo:        repository.NewNodeRepository(db),
		groupMemberRepo: repository.NewGroupMemberRepository(db),
		labelRepo:       repository.NewLabelRepository(db),
		groupNodeRepo:   repository.NewGroupNodeRepository(db),
		nodeLabelRepo:   repository.NewNodeLabelRepository(db),
		groupRepo:       repository.NewGroupRepository(db),
		baseRepo:        repository.NewNodeBaseRepository[entity.NodeLabel](db),
		logger:          log.NewLogger(log.Loglevel, "node-mapper"),
	}
}

func (n *nodeServiceImpl) Register(ctx context.Context, e *dto.NodeDto) (*entity.Node, error) {
	return nil, nil
}

func (n *nodeServiceImpl) CreateAppId(ctx context.Context) (*entity.Node, error) {
	username := ctx.Value("username")
	if username == nil {
		return nil, errors.New("invalid username")
	}
	userId := ctx.Value("userId")
	if userId == nil {
		return nil, errors.New("invalid userId")
	}

	peer := &entity.Node{
		AppID:     utils2.GenerateUUID(),
		UserId:    userId.(uint64),
		CreatedBy: username.(string),
	}

	err := n.nodeRepo.Create(ctx, peer)
	if err != nil {
		return nil, err
	}
	return peer, nil
}

func (n *nodeServiceImpl) Update(ctx context.Context, node *dto.NodeDto) error {
	return nil
}

func (n *nodeServiceImpl) UpdateStatus(ctx context.Context, nodeDto *dto.NodeDto) error {
	return n.nodeRepo.UpdateStatus(ctx, nodeDto)
}

func (n *nodeServiceImpl) DeleteNode(ctx context.Context, appId string) error {
	return n.nodeRepo.DeleteByAppId(ctx, appId)
}

func (n *nodeServiceImpl) GetByAppId(ctx context.Context, appId string) (*entity.Node, error) {
	return n.nodeRepo.FindByAppId(ctx, appId)
}

func (n *nodeServiceImpl) GetById(ctx context.Context, nodeId uint64) (*entity.Node, error) {
	return n.nodeRepo.Find(ctx, nodeId)
}

// GetNetMap params will filter
func (n *nodeServiceImpl) ListNodes(ctx context.Context, params *dto.QueryParams) (*vo.PageVo, error) {
	var nodes []*entity.Node
	result := new(vo.PageVo)
	nodes, count, err := n.nodeRepo.ListNodes(ctx, params)
	if err != nil {
		return nil, err
	}
	result.Total = count

	vos := transferToVos(nodes)
	result.Data = vos
	result.Page = params.Page
	result.Size = params.Size
	result.Current = params.Page

	return result, nil
}

// QueryNodes params will filter
func (n *nodeServiceImpl) QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*vo.NodeVo, error) {
	nodes, err := n.nodeRepo.QueryNodes(ctx, params)
	if err != nil {
		return nil, err
	}
	return transferToVos(nodes), nil
}

func transferToVos(nodes []*entity.Node) []*vo.NodeVo {
	var nodeVos []*vo.NodeVo
	for _, node := range nodes {
		nodeVo := &vo.NodeVo{
			ID:                  node.ID,
			Name:                node.Name,
			Description:         node.Description,
			CreatedBy:           node.CreatedBy,
			UserId:              node.UserId,
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
			ActiveStatus:        node.ActiveStatus,
			GroupName:           node.Group.GroupName,
			ConnectType:         node.ConnectType,
		}

		if node.NodeLabels != nil {
			labelResourceVo := new(vo.LabelResourceVo)
			labelResourceVo.LabelValues = make(map[string]string, 1)
			nodeVo.LabelResourceVo = labelResourceVo
			for _, label := range node.NodeLabels {
				labelResourceVo.LabelValues[fmt.Sprintf("%d", label.LabelId)] = label.LabelName
			}
		}
		nodeVos = append(nodeVos, nodeVo)
	}

	return nodeVos
}

// GetNetworkMap get user's network map
func (n *nodeServiceImpl) GetNetworkMap(ctx context.Context, appId, userId string) (*vo.NetworkMap, error) {
	var (
		groupNode  *entity.GroupNode
		groupNodes []*entity.GroupNode
		nodes      []*entity.Node
		err        error
	)
	current, err := n.nodeRepo.FindByAppId(ctx, appId)
	if err != nil {
		return nil, err
	}

	//find current node which group in
	groupNode, err = n.groupNodeRepo.FindByGroupNodeId(ctx, 0, current.ID)
	if err != nil {
		return nil, err
	}

	if groupNode == nil {
		return nil, nil
	}

	groupNodes, _, err = n.groupNodeRepo.List(ctx, &dto.GroupNodeParams{
		GroupID: groupNode.NetworkId,
	})

	var nodeIds []uint64
	for _, groupNode := range groupNodes {
		nodeIds = append(nodeIds, groupNode.NodeId)
	}

	// find nodes in group
	nodes, err = n.nodeRepo.FindIn(ctx, nodeIds)

	var resultNodes []*domain.Peer
	for _, node := range nodes {
		if node.Status == utils2.Online {
			resultNodes = append(resultNodes, &domain.Peer{
				Name:                node.Name,
				Description:         node.Description,
				NetworkId:           node.Group.NetworkId,
				Hostname:            node.Hostname,
				AppID:               node.AppID,
				Address:             node.Address,
				Endpoint:            node.Endpoint,
				PersistentKeepalive: node.PersistentKeepalive,
				PublicKey:           node.PublicKey,
				AllowedIPs:          node.AllowedIPs,
				Port:                node.Port,
				Status:              node.Status,
				GroupName:           node.Group.GroupName,
				DrpAddr:             node.DrpAddr,
				ConnectType:         node.ConnectType,
				Version:             0,
			})
		} else {
			n.logger.Verbosef("Peer %s is offline", node.AppID)
		}
	}

	return &vo.NetworkMap{
		UserId: userId,
		Current: &vo.NodeVo{
			ID:                  current.ID,
			Name:                current.Name,
			Description:         current.Description,
			NetworkID:           groupNode.NetworkId,
			CreatedBy:           current.CreatedBy,
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
			ConnectType:         current.ConnectType,
		},
		Nodes: resultNodes,
	}, nil

	//return nil, nil
}

// GroupVo Members
func (n *nodeServiceImpl) AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
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

	return n.groupMemberRepo.Create(ctx, member)
}

func (n *nodeServiceImpl) RemoveGroupMember(ctx context.Context, groupMemberId uint64) error {
	return n.groupMemberRepo.Delete(ctx, groupMemberId)
}

func (n *nodeServiceImpl) ListGroupMembers(ctx context.Context, params *dto.GroupMemberParams) (*vo.PageVo, error) {
	var (
		groupMember []*entity.GroupMember
		count       int64
		err         error
	)

	result := new(vo.PageVo)
	groupMember, count, err = n.groupMemberRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}
	result.Data = groupMember
	result.Page = params.Page
	result.Size = params.Size
	result.Current = params.Page
	result.Total = count

	return result, nil
}

func (n *nodeServiceImpl) UpdateGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error {
	return n.groupMemberRepo.Update(ctx, dto)
}

// Peer Tags
func (n *nodeServiceImpl) AddLabel(ctx context.Context, dto *dto.TagDto) error {
	label := strings.Split(dto.Label, ":")
	if len(label) != 2 || len(label[0]) == 0 || len(label[1]) == 0 {
		return errors.New("invalid label")
	}

	// TODO add label exists check

	return n.labelRepo.Create(ctx, &entity.Label{
		Label:     dto.Label,
		CreatedBy: dto.CreatedBy,
	})
}

func (n *nodeServiceImpl) UpdateLabel(ctx context.Context, dto *dto.TagDto) error {
	return n.labelRepo.Update(ctx, dto)
}

func (n *nodeServiceImpl) DeleteLabel(ctx context.Context, id uint64) error {
	return n.labelRepo.Delete(ctx, id)
}

func (n *nodeServiceImpl) ListLabel(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error) {
	var (
		labels []*entity.Label
		count  int64
		err    error
	)

	result := new(vo.PageVo)
	labels, count, err = n.labelRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	var labelVos []vo.LabelVo
	for _, label := range labels {
		labelVos = append(labelVos, vo.LabelVo{
			ID:        label.ID,
			Label:     label.Label,
			CreatedAt: label.CreatedAt,
			UpdatedAt: label.UpdatedAt,
			CreatedBy: label.CreatedBy,
			UpdatedBy: label.UpdatedBy,
		})
	}
	result.Data = labelVos
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size
	result.Total = count

	return result, nil
}

func (n *nodeServiceImpl) QueryLabels(ctx context.Context, params *dto.LabelParams) ([]*vo.LabelVo, error) {
	var (
		labels []*entity.Label
		err    error
	)

	labels, err = n.labelRepo.Query(ctx, params)
	if err != nil {
		return nil, err
	}

	// TODO add label exists check
	if len(labels) == 0 {
		return nil, errors.New("no label found")
	}

	var labelVos []*vo.LabelVo
	for _, label := range labels {
		labelVos = append(labelVos, &vo.LabelVo{
			ID:        label.ID,
			Label:     label.Label,
			CreatedAt: label.CreatedAt,
			UpdatedAt: label.UpdatedAt,
			CreatedBy: label.CreatedBy,
			UpdatedBy: label.UpdatedBy,
		})
	}

	return labelVos, nil
}

func (n *nodeServiceImpl) GetLabel(ctx context.Context, id uint64) (*entity.Label, error) {
	return n.labelRepo.Find(ctx, id)
}

// GroupVo Peer
func (n *nodeServiceImpl) AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error {
	groupNode := &entity.GroupNode{
		NetworkId: dto.NetworkId,
		NodeId:    dto.NodeID,
		GroupName: dto.NetworkName,
		CreatedBy: dto.CreatedBy,
	}
	return n.groupNodeRepo.Create(ctx, groupNode)
}

func (n *nodeServiceImpl) RemoveGroupNode(ctx context.Context, groupNodeId uint64) error {
	return n.groupNodeRepo.Delete(ctx, groupNodeId)
}

func (n *nodeServiceImpl) ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error) {
	var (
		groupNodes []*entity.GroupNode
		count      int64
		err        error
	)

	result := new(vo.PageVo)
	groupNodes, count, err = n.groupNodeRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}
	result.Data = groupNodes
	result.Page = params.Page
	result.Size = params.Size
	result.Current = params.Page
	result.Total = count

	return result, nil
}

func (n *nodeServiceImpl) GetGroupNode(ctx context.Context, groupNodeId uint64) (*entity.GroupNode, error) {
	return n.groupNodeRepo.Find(ctx, groupNodeId)
}

// Peer Label
func (n *nodeServiceImpl) AddNodeLabel(ctx context.Context, dto *dto.NodeLabelUpdateReq) error {

	needAddLabelId := strings.Split(dto.LabelIds, ",")
	if len(needAddLabelId) == 0 {
		return nil
	}
	for index, value := range needAddLabelId {
		fmt.Println("Index:", index, "Value:", value)
		labelId, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		var (
			label *entity.Label
		)
		label, err = n.labelRepo.Find(ctx, labelId)
		if label == nil {
			return errors.New("invalid label")
		}

		nodeId, err := strconv.ParseUint(dto.Id, 10, 64)
		if err != nil {
			return err
		}
		nodeLabel := &entity.NodeLabel{
			LabelId:   labelId,
			LabelName: label.Label,
			NodeId:    nodeId,
			CreatedBy: dto.CreatedBy,
		}

		if err = n.nodeLabelRepo.Create(ctx, nodeLabel); err != nil {
			return err
		}
	}
	return nil
}

func (n *nodeServiceImpl) RemoveNodeLabel(ctx context.Context, nodeId, labelId uint64) error {
	return n.nodeLabelRepo.DeleteByLabelId(ctx, nodeId, labelId)
}

func (n *nodeServiceImpl) ListNodeLabels(ctx context.Context, params *dto.NodeLabelParams) (*vo.PageVo, error) {
	var (
		nodeLabels []*entity.NodeLabel
		count      int64
		err        error
	)

	result := new(vo.PageVo)
	nodeLabels, count, err = n.nodeLabelRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}
	result.Total = count
	result.Page = params.Page
	result.Size = params.Size
	result.Current = params.Page
	result.Data = nodeLabels

	return result, nil
}

// all node apis
func (n *nodeServiceImpl) ListUserNodes(ctx context.Context, params *dto.ApiCommandParams) ([]vo.NodeVo, error) {
	var (
		nodes []*entity.Node
		err   error
	)
	userId := utils2.GetUserIdFromCtx(ctx)
	nodes, _, err = n.nodeRepo.ListNodes(ctx, &dto.QueryParams{
		UserId: userId,
	})

	if err != nil {
		return nil, err
	}

	var nodeVos []vo.NodeVo
	for _, node := range nodes {
		nodeVo := vo.NodeVo{
			ID:                  node.ID,
			Name:                node.Name,
			Description:         node.Description,
			CreatedBy:           node.CreatedBy,
			UserId:              node.UserId,
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
			GroupName:           node.Group.GroupName,
			ConnectType:         node.ConnectType,
		}
		nodeVos = append(nodeVos, nodeVo)
	}

	return nodeVos, nil
}

func (n *nodeServiceImpl) AddLabelToNode(ctx context.Context, params *dto.ApiCommandParams) error {
	return n.db.Transaction(func(tx *gorm.DB) error {
		appId := params.AppId
		node, err := n.nodeRepo.FindByAppId(ctx, appId)
		if err != nil {
			return fmt.Errorf("failed to find node by appId %s: %w", appId, err)
		}

		if node == nil {
			return fmt.Errorf("node with appId %s not found", appId)
		}
		_, count, err := n.nodeLabelRepo.WithTx(tx).List(ctx, &dto.NodeLabelParams{
			Label: params.Name,
		})

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to query label: %w", err)
		}

		if count > 0 {
			return fmt.Errorf("label %s already exists", params.Name)
		}

		//add
		labels, count, err := n.labelRepo.WithTx(tx).List(ctx, &dto.LabelParams{
			Label: params.Name,
		})

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to query label: %w", err)
		}

		if count == 0 {
			return fmt.Errorf("label %s not found", params.Name)
		}

		return n.nodeLabelRepo.WithTx(tx).Create(ctx, &entity.NodeLabel{
			LabelId:   labels[0].ID,
			LabelName: labels[0].Label,
			NodeId:    node.ID,
		})

	})
}

func (n *nodeServiceImpl) RemoveLabel(ctx context.Context, params *dto.ApiCommandParams) error {
	return n.db.Transaction(func(tx *gorm.DB) error {
		appId := params.AppId
		node, err := n.nodeRepo.FindByAppId(ctx, appId)
		if err != nil {
			return fmt.Errorf("failed to find node by appId %s: %w", appId, err)
		}

		if node == nil {
			return fmt.Errorf("node with appId %s not found", appId)
		}
		labels, count, err := n.nodeLabelRepo.WithTx(tx).List(ctx, &dto.NodeLabelParams{
			Label: params.Name,
		})

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to query label: %w", err)
		}

		if count == 0 {
			return fmt.Errorf("label %s not found", params.Name)
		}

		return n.nodeLabelRepo.WithTx(tx).DeleteByLabelId(ctx, node.ID, labels[0].ID)

	})
}

func (n *nodeServiceImpl) ShowLabel(ctx context.Context, params *dto.ApiCommandParams) ([]vo.NodeLabelVo, error) {
	appId := params.AppId
	node, err := n.nodeRepo.FindByAppId(ctx, appId)
	if err != nil {
		return nil, fmt.Errorf("failed to find node by appId %s: %w", appId, err)
	}

	if node == nil {
		return nil, fmt.Errorf("node with appId %s not found", appId)
	}
	labels, count, err := n.nodeLabelRepo.List(ctx, &dto.NodeLabelParams{
		NodeId: node.ID,
	})

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to query label: %w", err)
	}

	if count == 0 {
		return nil, nil
	}

	var vos []vo.NodeLabelVo
	for _, label := range labels {
		vos = append(vos, vo.NodeLabelVo{
			LabelName: label.LabelName,
			LabelId:   label.LabelId,
		})
	}

	return vos, nil
}
