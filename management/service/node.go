package service

import (
	"context"
	"errors"
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/repository"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// NodeService is an interface for peer mapper
type NodeService interface {
	Register(ctx context.Context, e *dto.NodeDto) (*entity.Node, error)
	CreateAppId(ctx context.Context) (*entity.Node, error)
	Update(ctx context.Context, e *dto.NodeDto) error
	DeleteNode(ctx context.Context, appId string) error

	// GetByAppId returns a peer by appId, every client has its own appId
	GetByAppId(ctx context.Context, appId string) (*entity.Node, error)

	GetById(ctx context.Context, nodeId uint64) (*entity.Node, error)

	GetNetworkMap(appId, userId string) (*vo.NetworkMap, error)

	// List returns a list of peers by userId，when client start up, it will call this method to get all the peers once
	// after that, it will call Watch method to get the latest peers
	ListNodes(ctx context.Context, params *dto.QueryParams) (*vo.PageVo, error)

	QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*vo.NodeVo, error)

	// Watch returns a channel that will be used to send the latest peers to the client
	//Watch() (<-chan *entity.Node, error)

	//GroupVo memeber
	AddGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error
	RemoveGroupMember(ctx context.Context, id uint64) error
	UpdateGroupMember(ctx context.Context, dto *dto.GroupMemberDto) error
	ListGroupMembers(ctx context.Context, params *dto.GroupMemberParams) (*vo.PageVo, error)

	//Node Label
	AddLabel(ctx context.Context, dto *dto.TagDto) error
	UpdateLabel(ctx context.Context, dto *dto.TagDto) error
	DeleteLabel(ctx context.Context, id uint64) error
	ListLabel(ctx context.Context, params *dto.LabelParams) (*vo.PageVo, error)
	QueryLabels(ctx context.Context, params *dto.LabelParams) ([]*vo.LabelVo, error)
	GetLabel(ctx context.Context, id uint64) (*entity.Label, error)

	//GroupVo Node
	AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error
	RemoveGroupNode(ctx context.Context, id uint64) error
	ListGroupNodes(ctx context.Context, params *dto.GroupNodeParams) (*vo.PageVo, error)
	GetGroupNode(ctx context.Context, id uint64) (*entity.GroupNode, error)

	//Node Label
	AddNodeLabel(ctx context.Context, dto *dto.NodeLabelUpdateReq) error
	RemoveNodeLabel(ctx context.Context, nodeId, labelId uint64) error
	ListNodeLabels(ctx context.Context, params *dto.NodeLabelParams) (*vo.PageVo, error)
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
		baseRepo:        repository.NewNodeBaseRepository[entity.NodeLabel](db),
		logger:          log.NewLogger(log.Loglevel, "peermapper")}
}

func (n *nodeServiceImpl) Register(ctx context.Context, e *dto.NodeDto) (*entity.Node, error) {
	count := n.GetAddress() + 1
	if count == -1 {
		return nil, errors.New("the address can not be allocated")
	}

	addressIP := fmt.Sprintf("10.0.%d.%d", (count-1)/254, ((count-1)%254)+1)

	peer := &entity.Node{
		Description:         e.Description,
		UserId:              e.UserID,
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
	err := n.nodeRepo.Create(ctx, peer)
	if err != nil {
		return nil, err
	}
	return peer, nil
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
		AppID:     utils.GenerateUUID(),
		UserId:    userId.(uint64),
		CreatedBy: username.(string),
	}

	err := n.nodeRepo.Create(ctx, peer)
	if err != nil {
		return nil, err
	}
	return peer, nil
}

func (n *nodeServiceImpl) Update(ctx context.Context, nodeDto *dto.NodeDto) error {
	return n.db.Transaction(func(tx *gorm.DB) error {
		var (
			err       error
			nodeLabes []entity.NodeLabel
		)

		// find current NodeLabel
		conditions := utils.NewQueryConditions()
		conditions.AddWhere("node_id", nodeDto.ID)
		query := conditions.BuildQuery(tx.Model(&entity.NodeLabel{}))
		if err = query.Find(&nodeLabes).Error; err != nil {
			return err
		}

		exists := make(map[uint64]bool, 1)
		for _, nodeLabel := range nodeLabes {
			if !exists[nodeLabel.LabelId] {
				exists[nodeLabel.LabelId] = true
			}
		}

		var allIds []interface{}
		if nodeDto.LabelIds != "" {
			ids := strings.Split(nodeDto.LabelIds, ",")
			for _, id := range ids {
				labelId, err := strconv.ParseUint(id, 10, 64)
				if err != nil {
					return err
				}
				if !exists[labelId] {
					allIds = append(allIds, labelId)
				}
			}

			// insert into new node labels
			var labels []entity.Label
			conditions := utils.NewQueryConditions()
			conditions.AddIn("id", allIds)
			query := conditions.BuildQuery(tx.Model(&entity.Label{}))
			if err = query.Find(&labels).Error; err != nil {
				return err
			}

			for _, label := range labels {
				nodeLabelRepo := n.nodeLabelRepo.WithTx(tx)
				if err = nodeLabelRepo.Create(ctx, &entity.NodeLabel{
					LabelId:   label.ID,
					LabelName: label.Label,
					NodeId:    nodeDto.ID,
				}); err != nil {
					return err
				}
			}
		}

		nodeRepo := n.nodeRepo.WithTx(tx)
		return nodeRepo.Update(ctx, nodeDto)
	})
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

// List params will filter
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
			//GroupName:           node.GroupName,
			//LabelValues:         labelValue,
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
func (n *nodeServiceImpl) GetNetworkMap(appId, userId string) (*vo.NetworkMap, error) {
	//current, _, err := n.GetByAppId(appId, "")
	//if err != nil {
	//	return nil, err
	//}
	//
	//var status = 1
	//peers, err := n.ListNodes(&dto.QueryParams{
	//	PubKey: &current.PublicKey,
	//	UserId: &userId,
	//	Status: &status,
	//})
	//
	//if err != nil {
	//	return nil, err
	//}
	//
	//return &vo.NetworkMap{
	//	UserId: userId,
	//	Peer: &vo.NodeVo{
	//		ID:                  current.ID,
	//		Name:                current.Name,
	//		Description:         current.Description,
	//		GroupId:             current.GroupId,
	//		CreatedBy:           current.CreatedBy,
	//		UserID:              current.UserID,
	//		Hostname:            current.Hostname,
	//		AppID:               current.AppID,
	//		Address:             current.Address,
	//		Endpoint:            current.Endpoint,
	//		PersistentKeepalive: current.PersistentKeepalive,
	//		PublicKey:           current.PublicKey,
	//		AllowedIPs:          current.AllowedIPs,
	//		RelayIP:             "",
	//		TieBreaker:          0,
	//		Ufrag:               "",
	//		Pwd:                 "",
	//		Port:                0,
	//		Status:              current.Status,
	//	},
	//	Peers: peers.Data,
	//}, nil

	return nil, nil
}

// GetAddress get peer address
func (n *nodeServiceImpl) GetAddress() int64 {
	return n.nodeRepo.GetAddress()
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

// Node Tags
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

// GroupVo Node
func (n *nodeServiceImpl) AddGroupNode(ctx context.Context, dto *dto.GroupNodeDto) error {
	groupNode := &entity.GroupNode{
		GroupId:   dto.GroupID,
		NodeId:    dto.NodeID,
		GroupName: dto.GroupName,
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

// Node Label
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
