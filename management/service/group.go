package service

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type GroupService interface {
	//Group
	GetNodeGroup(ctx context.Context, id string) (*vo.Group, error)
	CreateGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	DeleteGroup(ctx context.Context, id string) error
	ListGroups(ctx context.Context, params *dto.GroupParams) (*vo.PageVo, error)
	QueryGroups(ctx context.Context, params *dto.GroupParams) ([]*vo.Group, error)

	ListGroupPolicy(ctx context.Context, params *dto.GroupPolicyParams) ([]*vo.GroupPolicyVo, error)
	DeleteGroupPolicy(ctx context.Context, groupId uint, policyId uint) error
	DeleteGroupNode(ctx context.Context, groupId uint, nodeId uint) error
}

var (
	_ GroupService = (*groupServiceImpl)(nil)
)

type groupServiceImpl struct {
	logger *log.Logger
	*DatabaseService
	manager           *vo.WatchManager
	nodeServiceImpl   NodeService
	policyServiceImpl AccessPolicyService
}

func NewGroupService(db *DatabaseService) GroupService {
	return &groupServiceImpl{DatabaseService: db,
		logger:            log.NewLogger(log.Loglevel, "group-policy-service"),
		nodeServiceImpl:   NewNodeService(db),
		manager:           vo.NewWatchManager(),
		policyServiceImpl: NewAccessPolicyService(db),
	}
}

// NodeGroup
func (g *groupServiceImpl) GetNodeGroup(ctx context.Context, nodeId string) (*vo.Group, error) {
	var (
		group entity.NodeGroup
		err   error
	)

	if err = g.Model(&entity.NodeGroup{}).Where("id = ?", nodeId).First(&group).Error; err != nil {
		return nil, err
	}

	res, err := g.fetchNodeAndGroup(group.ID)

	return &vo.Group{
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
	return g.DB.Transaction(func(tx *gorm.DB) error {
		var group *entity.NodeGroup
		var err error
		if group, err = createGroupData(tx, dto); err != nil {
			return err
		}
		return g.handleGP(ctx, tx, dto, group)
	})

}

func createGroupData(tx *gorm.DB, dto *dto.NodeGroupDto) (*entity.NodeGroup, error) {
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

	if err := tx.Model(&entity.NodeGroup{}).Where("name = ? and created_by = ?", group.Name, group.CreatedBy).Count(&count).Error; err != nil {
		return nil, err
	}

	if count != 0 {
		return nil, errors.New("this group already exists")
	}

	if group.CreatedBy != "" {
		if err := tx.Where("username = ?", group.CreatedBy).First(&user).Error; err != nil {
			return nil, err
		}
		group.OwnerId = user.ID
	}
	err := tx.Create(&group).Error
	return group, err
}

func (g *groupServiceImpl) UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	return g.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		var group *entity.NodeGroup
		if group, err = updateGroupData(ctx, tx, dto); err != nil {
			return err
		}

		return g.handleGP(ctx, tx, dto, group)
	})

}

func updateGroupData(ctx context.Context, tx *gorm.DB, dto *dto.NodeGroupDto) (*entity.NodeGroup, error) {
	group := &entity.NodeGroup{
		Description: dto.Description,
		IsPublic:    dto.IsPublic,
		UpdatedBy:   dto.UpdatedBy,
	}

	if err := tx.Model(&entity.NodeGroup{}).Where("id = ?", dto.ID).Updates(group).Find(group).Error; err != nil {
		return nil, err
	}

	return group, nil
}

func (g *groupServiceImpl) handleGP(ctx context.Context, tx *gorm.DB, dto *dto.NodeGroupDto, group *entity.NodeGroup) error {

	var err error
	if dto.NodeIdList != nil {
		for _, nodeId := range dto.NodeIdList {
			var groupNode entity.GroupNode
			if err = tx.Model(&entity.GroupNode{}).Where("group_id = ? and node_id = ?", group.ID, nodeId).First(&groupNode).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					var node entity.Node
					if err = tx.Model(&entity.Node{}).Where("id = ?", nodeId).Find(&node).Error; err != nil {
						return err
					}

					groupNode = entity.GroupNode{
						GroupId:   group.ID,
						NodeId:    node.ID,
						GroupName: group.Name,
						NodeName:  node.Name,
						CreatedBy: ctx.Value("username").(string),
					}
					if err := tx.Model(&entity.GroupNode{}).Create(&groupNode).Error; err != nil {
						return err
					}

					// add push message
					g.manager.Push(node.PublicKey, &vo.Message{
						EventType: vo.EventTypeGroupAdd,
						GroupMessage: &vo.GroupMessage{
							GroupName: group.Name,
							GroupId:   group.ID,
						},
					})
				}
			}
		}
	}

	if dto.PolicyIdList != nil {
		for _, policyId := range dto.PolicyIdList {
			var groupPolicy entity.GroupPolicy
			if err = tx.Model(&entity.GroupPolicy{}).Where("group_id = ? and policy_id = ?", group.ID, policyId).First(&groupPolicy).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					var policy entity.AccessPolicy
					if err = tx.Model(&entity.AccessPolicy{}).Where("id = ?", policyId).Find(&policy).Error; err != nil {
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
	return g.DB.Transaction(func(tx *gorm.DB) error {
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
		groups   []entity.NodeGroup
		db       *gorm.DB
		groupVos []*vo.NodeGroupVo
	)

	result := new(vo.PageVo)
	db = g.DB

	sql, wrappers := utils.GenerateSql(params)
	if sql != "" {
		db = g.DB.Where(sql, wrappers...)
	}

	if err = db.Model(&entity.NodeGroup{}).Preload("GroupNodes").Preload("GroupPolicies").Count(&result.Total).Offset(params.Size * (params.Page - 1)).Limit(params.Size).Find(&groups).Error; err != nil {
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
	result.Size = params.Size

	return result, nil
}

func (g *groupServiceImpl) QueryGroups(ctx context.Context, params *dto.GroupParams) ([]*vo.Group, error) {
	var nodeGroups []entity.NodeGroup

	sql, wrappers := utils.Generate(params)
	db := g.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	g.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := db.Model(&entity.NodeGroup{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&nodeGroups).Error; err != nil {
		return nil, err
	}

	var nodeVos []*vo.Group
	for _, group := range nodeGroups {
		res, err := g.fetchNodeAndGroup(group.ID)
		if err != nil {
			return nil, err
		}
		nodeVos = append(nodeVos, &vo.Group{
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

func (g *groupServiceImpl) fetchNodeAndGroup(groupId uint) (*vo.GroupRelationVo, error) {
	// query group node
	var groupNodes []entity.GroupNode
	var err error
	if err = g.Model(&entity.GroupNode{}).Where("group_id = ?", groupId).Find(&groupNodes).Error; err != nil {
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
	var groupPolicies []entity.GroupPolicy
	if err = g.Model(&entity.GroupPolicy{}).Where("group_id = ?", groupId).Find(&groupPolicies).Error; err != nil {
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
	var groupPolicies []*entity.GroupPolicy
	if err := g.Model(&entity.GroupPolicy{}).Where("group_id = ?", params.GroupId).Find(&groupPolicies).Error; err != nil {
		return nil, err
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

func (g *groupServiceImpl) DeleteGroupPolicy(ctx context.Context, groupId uint, policyId uint) error {
	return g.Model(&entity.GroupPolicy{}).Where("group_id = ? and policy_id = ?", groupId, policyId).Delete(&entity.GroupPolicy{}).Error
}

func (g *groupServiceImpl) DeleteGroupNode(ctx context.Context, groupId uint, nodeId uint) error {
	return g.DB.Transaction(func(tx *gorm.DB) error {
		var groupNode entity.GroupNode
		if err := g.Model(&entity.GroupNode{}).Where("group_id = ? and node_id = ?", groupId, nodeId).Delete(&groupNode).Error; err != nil {
			return err
		}

		var node entity.Node
		if err := g.Model(&entity.Node{}).Where("id = ?", nodeId).Find(&node).Error; err != nil {
			return err
		}

		g.manager.Push(node.PublicKey, &vo.Message{
			EventType: vo.EventTypeGroupRemove,
			GroupMessage: &vo.GroupMessage{
				GroupId:   groupId,
				GroupName: groupNode.GroupName,
			},
		})

		return nil
	})
}
