package service

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
	"strconv"
)

type GroupService interface {
	//Group
	GetNodeGroup(ctx context.Context, id string) (*vo.NodeGroupVo, error)
	CreateGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	DeleteGroup(ctx context.Context, id string) error
	ListGroups(ctx context.Context, params *dto.GroupParams) (*vo.PageVo, error)

	ListGroupPolicy(ctx context.Context, params *dto.GroupPolicyParams) ([]*vo.GroupPolicyVo, error)
	DeleteGroupPolicy(groupId uint, policyId uint) error
}

var (
	_ GroupService = (*groupServiceImpl)(nil)
)

type groupServiceImpl struct {
	logger *log.Logger
	*DatabaseService
}

func NewGroupService(db *DatabaseService) GroupService {
	return &groupServiceImpl{DatabaseService: db,
		logger: log.NewLogger(log.Loglevel, "[group-policy-service] "),
	}
}

// NodeGroup
func (g *groupServiceImpl) GetNodeGroup(ctx context.Context, nodeId string) (*vo.NodeGroupVo, error) {
	var (
		group entity.NodeGroup
		err   error
	)

	if err = g.Where("id = ?", nodeId).First(&group).Error; err != nil {
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

func (g *groupServiceImpl) CreateGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
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

	if err := g.Model(&entity.NodeGroup{}).Where("name = ? and created_by = ?", group.Name, group.CreatedBy).Count(&count).Error; err != nil {
		return err
	}

	if count != 0 {
		return errors.New("this group already exists")
	}

	if group.CreatedBy != "" {
		if err := g.Where("username = ?", group.CreatedBy).First(&user).Error; err != nil {
			return err
		}
		group.OwnerID = user.ID
	}
	if err := g.Create(group).Error; err != nil {
		return err
	}

	if dto.Nodes != nil {
		for _, nodeId := range dto.Nodes {
			var groupNode *entity.GroupNode
			if err := g.Model(&entity.GroupNode{}).Where("group_id = ? and node_id = ?", group.ID, nodeId).First(groupNode).Error; err != nil {

			}

			if groupNode != nil {
				continue
			}
			id, err := strconv.Atoi(nodeId)
			if err != nil {
				return errors.New("Invalid id")
			}
			groupNode = &entity.GroupNode{
				GroupID:   group.ID,
				NodeID:    uint(id),
				GroupName: group.Name,
				NodeName:  "",
				CreatedBy: ctx.Value("username").(string),
			}
			if err := g.Model(&entity.GroupNode{}).Create(&groupNode).Error; err != nil {
				return err
			}
		}
	}

	if dto.Policies != nil {
		for _, policyId := range dto.Policies {
			var groupPolicy *entity.GroupPolicy
			if err := g.Model(&entity.GroupPolicy{}).Where("group_id = ? and policy_id = ?", group.ID, policyId).First(groupPolicy).Error; err != nil {

			}

			if groupPolicy != nil {
				continue
			}
			id, err := strconv.Atoi(policyId)
			if err != nil {
				return errors.New("Invalid id")
			}
			groupPolicy = &entity.GroupPolicy{
				GroupID:     group.ID,
				PolicyId:    uint(id),
				PolicyName:  "",
				Description: "",
				CreatedBy:   ctx.Value("username").(string),
			}
			if err := g.Model(&entity.GroupPolicy{}).Create(&groupPolicy).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *groupServiceImpl) UpdateGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	group := &entity.NodeGroup{
		Description: dto.Description,
		IsPublic:    dto.IsPublic,
		UpdatedBy:   dto.UpdatedBy,
	}

	if err := g.Model(&entity.NodeGroup{}).Where("id = ?", dto.ID).Updates(group).Error; err != nil {
		return err
	}

	if dto.Nodes != nil {
		for _, nodeId := range dto.Nodes {
			var groupNode *entity.GroupNode
			if err := g.Model(&entity.GroupNode{}).Where("group_id = ? and node_id = ?", dto.ID, nodeId).First(groupNode).Error; err != nil {

			}

			if groupNode != nil {
				continue
			}
			id, err := strconv.Atoi(nodeId)
			if err != nil {
				return errors.New("Invalid id")
			}
			groupNode = &entity.GroupNode{
				GroupID:   dto.ID,
				NodeID:    uint(id),
				GroupName: group.Name,
				NodeName:  "",
				CreatedBy: ctx.Value("username").(string),
			}
			if err := g.Model(&entity.GroupNode{}).Create(&groupNode).Error; err != nil {
				return err
			}
		}
	}

	if dto.Policies != nil {
		for _, policyId := range dto.Policies {
			var groupPolicy entity.GroupPolicy
			if err := g.Model(&entity.GroupPolicy{}).Where("group_id = ? and policy_id = ?", dto.ID, policyId).First(&groupPolicy).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					id, err := strconv.Atoi(policyId)
					if err != nil {
						return errors.New("Invalid id")
					}
					groupPolicy = entity.GroupPolicy{
						GroupID:     dto.ID,
						PolicyId:    uint(id),
						PolicyName:  "",
						Description: "",
						CreatedBy:   ctx.Value("username").(string),
					}
					if err := g.Model(&entity.GroupPolicy{}).Create(&groupPolicy).Error; err != nil {
						return err
					}
				}
			}

		}
	}

	return nil
}

func (g *groupServiceImpl) DeleteGroup(ctx context.Context, id string) error {
	if err := g.Where("id = ?", id).Delete(&entity.NodeGroup{}).Error; err != nil {
		return err
	}
	return nil
}

func (g *groupServiceImpl) ListGroups(ctx context.Context, params *dto.GroupParams) (*vo.PageVo, error) {
	var nodeGroups []entity.NodeGroup

	result := new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := g.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	if err := db.Model(&entity.NodeGroup{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	g.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
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

func (g groupServiceImpl) ListGroupPolicy(ctx context.Context, params *dto.GroupPolicyParams) ([]*vo.GroupPolicyVo, error) {
	var groupPolicies []*entity.GroupPolicy
	if err := g.Model(&entity.GroupPolicy{}).Where("group_id = ?", params.GroupId).Find(&groupPolicies).Error; err != nil {
		return nil, err
	}

	var groupPolicyVos []*vo.GroupPolicyVo
	for _, groupPolicy := range groupPolicies {
		groupPolicyVos = append(groupPolicyVos, &vo.GroupPolicyVo{
			ID:          groupPolicy.ID,
			GroupId:     groupPolicy.GroupID,
			PolicyId:    groupPolicy.PolicyId,
			PolicyName:  groupPolicy.PolicyName,
			Description: groupPolicy.Description,
		})
	}
	return groupPolicyVos, nil
}

func (g groupServiceImpl) DeleteGroupPolicy(groupId uint, policyId uint) error {
	return g.Model(&entity.GroupPolicy{}).Where("group_id = ? and id = ?", groupId, policyId).Delete(&entity.GroupPolicy{}).Error
}
