package service

import (
	"context"
	"errors"
	"fmt"
	"linkany/internal"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/repository"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
	"strconv"

	"gorm.io/gorm"
)

type GroupService interface {
	//GroupVo
	GetNodeGroup(ctx context.Context, id uint64) (*vo.GroupVo, error)
	CreateGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	DeleteGroup(ctx context.Context, id string) error
	ListGroups(ctx context.Context, params *dto.GroupParams) (*vo.PageVo, error)
	QueryGroups(ctx context.Context, params *dto.GroupParams) ([]*vo.GroupVo, error)

	ListGroupPolicy(ctx context.Context, params *dto.GroupPolicyParams) ([]*vo.GroupPolicyVo, error)
	DeleteGroupPolicy(ctx context.Context, groupId, policyId uint64) error
	DeleteGroupNode(ctx context.Context, groupId, nodeId uint64) error

	//api
	JoinGroup(ctx context.Context, params *dto.ApiCommandParams) error
	LeaveGroup(ctx context.Context, params *dto.ApiCommandParams) error
	RemoveGroup(ctx context.Context, params *dto.ApiCommandParams) error
	AddGroup(ctx context.Context, params *dto.ApiCommandParams) error
}

var (
	_ GroupService = (*groupServiceImpl)(nil)
)

type groupServiceImpl struct {
	db              *gorm.DB
	logger          *log.Logger
	manager         *internal.WatchManager
	nodeRepo        repository.NodeRepository
	groupRepo       repository.GroupRepository
	groupNodeRepo   repository.GroupNodeRepository
	groupPolicyRepo repository.GroupPolicyRepository
	policyRepo      repository.PolicyRepository
	//policyServiceImpl AccessPolicyService
}

func NewGroupService(db *gorm.DB) GroupService {
	return &groupServiceImpl{
		db:              db,
		logger:          log.NewLogger(log.Loglevel, "group-policy-service"),
		nodeRepo:        repository.NewNodeRepository(db),
		groupRepo:       repository.NewGroupRepository(db),
		groupNodeRepo:   repository.NewGroupNodeRepository(db),
		groupPolicyRepo: repository.NewGroupPolicyRepository(db),
		policyRepo:      repository.NewPolicyRepository(db),
		manager:         internal.NewWatchManager(),
	}
}

// NodeGroup
func (g *groupServiceImpl) GetNodeGroup(ctx context.Context, nodeId uint64) (*vo.GroupVo, error) {
	var (
		group *entity.NodeGroup
		err   error
	)

	if group, err = g.groupRepo.Find(ctx, nodeId); err != nil {
		return nil, err
	}

	res, err := g.fetchNodeAndGroup(ctx, group.ID)

	return &vo.GroupVo{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		//NodeCount:   len(groupNodes),
		GroupRelationVo: res,
		CreatedAt:       group.CreatedAt,
		DeletedAt:       group.DeletedAt,
		UpdatedAt:       group.UpdatedAt,
		CreatedBy:       group.CreatedBy,
		UpdatedBy:       group.UpdatedBy,
	}, nil
}

func (g *groupServiceImpl) CreateGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	return g.db.Transaction(func(tx *gorm.DB) error {
		var group *entity.NodeGroup
		var err error
		if group, err = g.createGroupData(ctx, tx, dto); err != nil {
			return err
		}
		return g.handleGP(ctx, tx, dto, group)
	})

}

func (g *groupServiceImpl) createGroupData(ctx context.Context, tx *gorm.DB, dto *dto.NodeGroupDto) (*entity.NodeGroup, error) {
	group := &entity.NodeGroup{
		Name:        dto.Name,
		Description: dto.Description,
		IsPublic:    dto.IsPublic,
		CreatedBy:   dto.CreatedBy,
		UpdatedBy:   dto.CreatedBy,
	}

	groupRepo := g.groupRepo.WithTx(tx)

	group, err := groupRepo.FindByName(ctx, group.Name)
	if err != nil {
		return nil, err
	}

	if group != nil {
		return nil, fmt.Errorf("group name already exists")
	}

	return group, groupRepo.Create(ctx, group)

}

func (g *groupServiceImpl) UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	return g.db.Transaction(func(tx *gorm.DB) error {
		var (
			err   error
			group *entity.NodeGroup
		)

		if group, err = g.updateGroupData(ctx, tx, dto); err != nil {
			return err
		}

		return g.handleGP(ctx, tx, dto, group)
	})

}

func (g *groupServiceImpl) updateGroupData(ctx context.Context, tx *gorm.DB, dto *dto.NodeGroupDto) (*entity.NodeGroup, error) {
	return g.groupRepo.Update(ctx, dto)
}

func (g *groupServiceImpl) handleGP(ctx context.Context, tx *gorm.DB, dto *dto.NodeGroupDto, group *entity.NodeGroup) error {

	var err error
	if dto.NodeIdList != nil {
		for _, nodeIdStr := range dto.NodeIdList {
			var (
				groupNode entity.GroupNode
			)
			// check if group node exists
			nodeId, err := strconv.ParseUint(nodeIdStr, 10, 64)
			if err != nil {
				return err
			}
			if gn, err := g.groupNodeRepo.FindByGroupNodeId(ctx, group.ID, nodeId); err != nil || gn == nil {
				//if errors.Is(err, gorm.ErrRecordNotFound) {
				var node *entity.Node
				if node, err = g.nodeRepo.Find(ctx, nodeId); err != nil {
					return err
				}

				groupNode = entity.GroupNode{
					GroupId:   group.ID,
					NodeId:    node.ID,
					GroupName: group.Name,
					NodeName:  node.Name,
					CreatedBy: ctx.Value("username").(string),
				}
				if err = g.groupNodeRepo.Create(ctx, &groupNode); err != nil {
					return err
				}

				// add push message
				g.manager.Push(node.PublicKey, internal.NewMessage().AddNode(
					node.TransferToNodeVo().TransferToNodeMessage(),
				))
				//}
			}
		}
	}

	if dto.PolicyIdList != nil {
		for _, policyId := range dto.PolicyIdList {
			var groupPolicy entity.GroupPolicy
			if err = tx.Model(&entity.GroupPolicy{}).Where("group_id = ? and policy_id = ?", group.ID, policyId).First(&groupPolicy).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					var (
						policy *entity.AccessPolicy
						err    error
						poId   uint64
					)
					policyRepo := g.policyRepo.WithTx(tx)
					if poId, err = strconv.ParseUint(policyId, 10, 64); err != nil {
						return err
					}
					if policy, err = policyRepo.Find(ctx, poId); err != nil {
						return err
					}

					groupPolicy = entity.GroupPolicy{
						GroupId:    group.ID,
						PolicyId:   policy.ID,
						PolicyName: policy.Name,
						CreatedBy:  ctx.Value("username").(string),
					}
					if err := tx.Model(&entity.GroupPolicy{}).Create(&groupPolicy).Error; err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (g *groupServiceImpl) DeleteGroup(ctx context.Context, id string) error {
	return g.db.Transaction(func(tx *gorm.DB) error {
		var err error
		if err = tx.Model(&entity.NodeGroup{}).Where("id = ?", id).Delete(&entity.NodeGroup{}).Error; err != nil {
			return err
		}

		if err = tx.Model(&entity.GroupNode{}).Where("group_id = ?", id).Delete(&entity.GroupNode{}).Error; err != nil {
			return err
		}

		if err = tx.Model(&entity.GroupPolicy{}).Where("group_id = ?", id).Delete(&entity.GroupPolicy{}).Error; err != nil {
			return err
		}

		return nil
	})

}

func (g *groupServiceImpl) ListGroups(ctx context.Context, params *dto.GroupParams) (*vo.PageVo, error) {
	var (
		err      error
		groups   []*entity.NodeGroup
		count    int64
		groupVos []*vo.NodeGroupVo
	)

	result := new(vo.PageVo)
	groups, count, err = g.groupRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		groupVo := &vo.NodeGroupVo{
			GroupRelationVo: nil,
			ModelVo: vo.ModelVo{
				ID:        group.ID,
				CreatedAt: group.CreatedAt,
				UpdatedAt: group.UpdatedAt,
				DeletedAt: group.DeletedAt.Time,
			},
			Name:        group.Name,
			NodeCount:   0,
			Status:      group.Status.String(),
			Description: group.Description,
		}

		groupRelationVo := vo.NewGroupRelationVo()
		groupVo.GroupRelationVo = groupRelationVo

		// fill group nodes
		if len(group.GroupNodes) > 0 {
			groupVo.GroupNodes = make([]vo.GroupNodeVo, 0)
			for _, node := range group.GroupNodes {
				groupVo.GroupNodes = append(groupVo.GroupNodes, vo.GroupNodeVo{
					NodeId:   node.NodeId,
					NodeName: node.NodeName,
				})
				groupVo.NodeCount++
				groupRelationVo.NodeValues[fmt.Sprintf("%d", node.NodeId)] = node.NodeName // for tom-select show, use nodeId as key
			}

		}

		// fill group policies
		if len(group.GroupPolicies) > 0 {
			groupVo.GroupPolicies = make([]vo.GroupPolicyVo, 0)
			for _, policy := range group.GroupPolicies {
				groupVo.GroupPolicies = append(groupVo.GroupPolicies, vo.GroupPolicyVo{
					PolicyId:   policy.PolicyId,
					PolicyName: policy.PolicyName,
				})
				groupVo.NodeCount++

				groupRelationVo.PolicyValues[fmt.Sprintf("%d", policy.PolicyId)] = policy.PolicyName // for tom-select show, use policyId as key
			}
		}

		groupVos = append(groupVos, groupVo)

	}

	result.Data = groupVos
	result.Page = params.Page
	result.Current = params.Page
	result.Size = params.Size
	result.Total = count

	return result, nil
}

func (g *groupServiceImpl) QueryGroups(ctx context.Context, params *dto.GroupParams) ([]*vo.GroupVo, error) {
	var (
		err        error
		nodeGroups []*entity.NodeGroup
	)

	if nodeGroups, err = g.groupRepo.Query(ctx, params); err != nil {
		return nil, err
	}

	var nodeVos []*vo.GroupVo
	for _, group := range nodeGroups {
		res, err := g.fetchNodeAndGroup(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		nodeVos = append(nodeVos, &vo.GroupVo{
			ID:              group.ID,
			Name:            group.Name,
			Description:     group.Description,
			GroupRelationVo: res,
			CreatedAt:       group.CreatedAt,
			DeletedAt:       group.DeletedAt,
			UpdatedAt:       group.UpdatedAt,
			CreatedBy:       group.CreatedBy,
			UpdatedBy:       group.UpdatedBy,
		})
	}

	return nodeVos, nil
}

func (g *groupServiceImpl) fetchNodeAndGroup(ctx context.Context, groupId uint64) (*vo.GroupRelationVo, error) {
	// query group node
	var (
		groupNodes []*entity.GroupNode
		err        error
	)
	//if err = g.Model(&entity.GroupNode{}).Where("group_id = ?", groupId).Find(&groupNodes).Error; err != nil {
	//	if !errors.Is(err, gorm.ErrRecordNotFound) {
	//		return nil, err
	//	}
	//}

	if groupNodes, _, err = g.groupNodeRepo.List(ctx, &dto.GroupNodeParams{
		GroupID: groupId,
	}); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	result := new(vo.GroupRelationVo)
	nodeResourceVo := new(vo.NodeResourceVo)

	nodeValues := make(map[string]string, 1)
	for _, groupNode := range groupNodes {
		nodeValues[fmt.Sprintf("%d", groupNode.NodeId)] = groupNode.NodeName
	}
	nodeResourceVo.NodeValues = nodeValues
	result.NodeResourceVo = nodeResourceVo

	// query policies
	var groupPolicies []*entity.GroupPolicy

	if groupPolicies, _, err = g.groupPolicyRepo.List(ctx, &dto.GroupPolicyParams{
		GroupId: groupId,
	}); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	policyResourceVo := new(vo.PolicyResourceVo)
	policyValues := make(map[string]string, 1)
	for _, groupPolicy := range groupPolicies {
		policyValues[fmt.Sprintf("%d", groupPolicy.PolicyId)] = groupPolicy.PolicyName
	}

	policyResourceVo.PolicyValues = policyValues
	result.PolicyResourceVo = policyResourceVo

	return result, nil
}

func (g *groupServiceImpl) ListGroupPolicy(ctx context.Context, params *dto.GroupPolicyParams) ([]*vo.GroupPolicyVo, error) {
	var (
		err           error
		groupPolicies []*entity.GroupPolicy
	)

	if groupPolicies, _, err = g.groupPolicyRepo.List(ctx, params); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	var groupPolicyVos []*vo.GroupPolicyVo
	for _, groupPolicy := range groupPolicies {
		groupPolicyVos = append(groupPolicyVos, &vo.GroupPolicyVo{
			ModelVo: vo.ModelVo{
				ID: groupPolicy.ID,
			},
			GroupId:     groupPolicy.GroupId,
			PolicyId:    groupPolicy.PolicyId,
			PolicyName:  groupPolicy.PolicyName,
			Description: groupPolicy.Description,
		})
	}
	return groupPolicyVos, nil
}

func (g *groupServiceImpl) DeleteGroupPolicy(ctx context.Context, groupId, policyId uint64) error {
	//return g.Model(&entity.GroupPolicy{}).Where("group_id = ? and policy_id = ?", groupId, policyId).Delete(&entity.GroupPolicy{}).Error
	return g.groupPolicyRepo.DeleteByGroupPolicyId(ctx, groupId, policyId)
}

func (g *groupServiceImpl) DeleteGroupNode(ctx context.Context, groupId, nodeId uint64) error {
	return g.db.Transaction(func(tx *gorm.DB) error {
		var (
			err error
		)

		groupNodeRepo := g.groupNodeRepo.WithTx(tx)
		if err = groupNodeRepo.DeleteByGroupNodeId(ctx, groupId, nodeId); err != nil {
			return err
		}

		nodeRepo := g.nodeRepo.WithTx(tx)
		node, err := nodeRepo.Find(ctx, nodeId)
		if err != nil {
			return err
		}

		g.manager.Push(node.PublicKey, internal.NewMessage().RemoveNode(
			node.TransferToNodeVo().TransferToNodeMessage(),
		))

		return nil
	})
}

// api groups

func (g *groupServiceImpl) JoinGroup(ctx context.Context, params *dto.ApiCommandParams) error {
	return g.db.Transaction(func(tx *gorm.DB) error {
		var (
			err       error
			group     *entity.NodeGroup
			node      *entity.Node
			groupNode *entity.GroupNode
		)
		// find group
		if group, err = g.groupRepo.WithTx(tx).FindByName(ctx, params.Name); err != nil {
			return err
		}

		if group == nil {
			return fmt.Errorf("group %s not found", params.Name)
		}

		if node, err = g.nodeRepo.WithTx(tx).FindByAppId(ctx, params.AppId); err != nil {
			return err
		}

		if groupNode, err = g.groupNodeRepo.WithTx(tx).FindByGroupNodeId(ctx, group.ID, node.ID); err != nil {
			return err
		}

		if groupNode != nil {
			return fmt.Errorf("node %s already in group %s, please leave first", node.Name, group.Name)
		} else {
			//create
			return g.groupNodeRepo.WithTx(tx).Create(ctx, &entity.GroupNode{
				GroupId:   group.ID,
				NodeId:    node.ID,
				GroupName: group.Name,
				NodeName:  node.Name,
			})
		}

		return nil
	})
}

func (g *groupServiceImpl) LeaveGroup(ctx context.Context, params *dto.ApiCommandParams) error {
	return g.db.Transaction(func(tx *gorm.DB) error {
		var (
			err   error
			group *entity.NodeGroup
			node  *entity.Node
		)
		// find group
		if group, err = g.groupRepo.WithTx(tx).FindByName(ctx, params.Name); err != nil {
			return err
		}

		if group == nil {
			return fmt.Errorf("group %s not found", params.Name)
		}

		if node, err = g.nodeRepo.WithTx(tx).FindByAppId(ctx, params.AppId); err != nil {
			return err
		}

		return g.groupNodeRepo.DeleteByGroupNodeId(ctx, group.ID, node.ID)

	})
}

func (g *groupServiceImpl) RemoveGroup(ctx context.Context, params *dto.ApiCommandParams) error {
	return g.db.Transaction(func(tx *gorm.DB) error {
		var (
			err   error
			group *entity.NodeGroup
		)
		// find group
		if group, err = g.groupRepo.WithTx(tx).FindByName(ctx, params.Name); err != nil {
			return err
		}

		if group == nil {
			return fmt.Errorf("group %s not found", params.Name)
		}

		return g.groupRepo.WithTx(tx).Delete(ctx, group.ID)
	})
}

func (g *groupServiceImpl) AddGroup(ctx context.Context, params *dto.ApiCommandParams) error {
	return g.db.Transaction(func(tx *gorm.DB) error {
		var (
			err   error
			group *entity.NodeGroup
		)

		// find group
		if group, err = g.groupRepo.WithTx(tx).FindByName(ctx, params.Name); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		if group != nil {
			return fmt.Errorf("group %s already exists", params.Name)
		}

		return g.groupRepo.WithTx(tx).Create(ctx, &entity.NodeGroup{
			Name:  params.Name,
			OwnId: utils.GetUserIdFromCtx(ctx),
		})

	})
}
