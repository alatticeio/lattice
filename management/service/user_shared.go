package service

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type SharedService interface {

	// Shared Group
	GetSharedGroup(ctx context.Context, id string) (*vo.SharedNodeGroupVo, error)
	ListSharedGroup(ctx context.Context, userId string) ([]*vo.SharedNodeGroupVo, error)
	CreateSharedGroup(ctx context.Context, dto *dto.SharedGroupDto) error
	DeleteSharedGroup(ctx context.Context, inviteId, groupId uint) error

	AddNodeToGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	AddPolicyToGroup(ctx context.Context, dto *dto.NodeGroupDto) error
	ListGroups(ctx context.Context, params *dto.SharedGroupParams) (*vo.PageVo, error)
	ListNodes(ctx context.Context, params *dto.SharedNodeParams) (*vo.PageVo, error)

	// Shared Policy
	GetSharedPolicy(ctx context.Context, id string) (*vo.SharedPolicyVo, error)
	CreateSharedPolicy(ctx context.Context, dto *dto.SharedPolicyDto) error
	UpdateSharedPolicy(ctx context.Context, dto *dto.SharedPolicyDto) error
	DeleteSharedPolicy(ctx context.Context, inviteId, policyId uint) error

	// Shared Node
	GetSharedNode(ctx context.Context, id string) (*vo.SharedNodeVo, error)
	CreateSharedNode(ctx context.Context, dto *dto.SharedNodeDto) error
	UpdateSharedNode(ctx context.Context, dto *dto.SharedNodeDto) error
	DeleteSharedNode(ctx context.Context, inviteId, id uint) error

	// Shared Label
	GetSharedLabel(ctx context.Context, id string) (*vo.SharedLabelVo, error)
	CreateSharedLabel(ctx context.Context, dto *dto.SharedLabelDto) error
	UpdateSharedLabel(ctx context.Context, dto *dto.SharedLabelDto) error
	DeleteSharedLabel(ctx context.Context, inviteId, id uint) error
}

var _ SharedService = (*shareServiceImpl)(nil)

type shareServiceImpl struct {
	*DatabaseService
	logger *log.Logger

	groupService  GroupService
	policyService AccessPolicyService
}

func NewSharedService(db *DatabaseService) SharedService {
	return &shareServiceImpl{
		DatabaseService: db,
		logger:          log.NewLogger(log.Loglevel, "SharedService"),
		groupService:    NewGroupService(db),
		policyService:   NewAccessPolicyService(db),
	}
}

func (s *shareServiceImpl) GetSharedGroup(ctx context.Context, id string) (*vo.SharedNodeGroupVo, error) {
	var (
		sharedGroup vo.SharedNodeGroupVo
		err         error
	)
	if err = s.Model(&sharedGroup).Where("id = ?", id).First(&sharedGroup).Error; err != nil {
		return nil, err
	}

	return &sharedGroup, nil
}

func (s *shareServiceImpl) ListSharedGroup(ctx context.Context, userId string) ([]*vo.SharedNodeGroupVo, error) {
	var (
		sharedGroups []*vo.SharedNodeGroupVo
		err          error
	)
	if err = s.Model(&sharedGroups).Where("user_id = ?", userId).Find(&sharedGroups).Error; err != nil {
		return nil, err
	}

	return sharedGroups, nil
}

func (s *shareServiceImpl) CreateSharedGroup(ctx context.Context, dto *dto.SharedGroupDto) error {
	sharedGroup := &entity.SharedNodeGroup{}

	if err := s.Create(sharedGroup).Error; err != nil {
		return err
	}
	return nil
}

func (s *shareServiceImpl) DeleteSharedGroup(ctx context.Context, inviteId, groupId uint) error {
	var (
		err error
	)
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err = s.Model(&entity.SharedNodeGroup{}).Where("invite_id = ? and group_id = ?", inviteId, groupId).Delete(&entity.SharedNodeGroup{}).Error; err != nil {
			return err
		}

		if err = s.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ? and resource_id  = ?", inviteId, groupId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
			return err
		}

		return nil
	})

}

func (s *shareServiceImpl) GetSharedPolicy(ctx context.Context, id string) (*vo.SharedPolicyVo, error) {
	var (
		sharedPolicy vo.SharedPolicyVo
		err          error
	)

	if err = s.Model(&entity.SharedPolicy{}).Where("id = ?", id).First(&sharedPolicy).Error; err != nil {
		return nil, err
	}

	return &sharedPolicy, nil
}

func (s *shareServiceImpl) CreateSharedPolicy(ctx context.Context, dto *dto.SharedPolicyDto) error {
	sharedPolicy := &vo.SharedPolicyVo{
		ID:          dto.ID,
		UserId:      dto.UserId,
		PolicyId:    dto.PolicyId,
		OwnerId:     dto.OwnerId,
		Description: dto.Description,
		GrantedAt:   dto.GrantedAt,
		RevokedAt:   dto.RevokedAt,
	}

	if err := s.Create(sharedPolicy).Error; err != nil {
		return err
	}
	return nil
}

func (s *shareServiceImpl) UpdateSharedPolicy(ctx context.Context, dto *dto.SharedPolicyDto) error {
	sharedPolicy := &vo.SharedPolicyVo{
		ID:          dto.ID,
		UserId:      dto.UserId,
		PolicyId:    dto.PolicyId,
		OwnerId:     dto.OwnerId,
		Description: dto.Description,
		GrantedAt:   dto.GrantedAt,
		RevokedAt:   dto.RevokedAt,
	}

	if err := s.Save(sharedPolicy).Error; err != nil {
		return err
	}
	return nil
}

func (s *shareServiceImpl) DeleteSharedPolicy(ctx context.Context, inviteId, policyId uint) error {
	var (
		err error
	)

	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err = s.Model(&entity.SharedPolicy{}).Where("invite_id and policy_id = ?", inviteId, policyId).Delete(&entity.SharedPolicy{}).Error; err != nil {
			return err
		}

		if err = s.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ? and resource_id  = ?", inviteId, policyId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
			return err
		}

		return nil
	})
}

func (s *shareServiceImpl) GetSharedNode(ctx context.Context, id string) (*vo.SharedNodeVo, error) {
	var (
		sharedNode vo.SharedNodeVo
		err        error
	)
	if err = s.Model(&sharedNode).Where("id = ?", id).First(&sharedNode).Error; err != nil {
		return nil, err
	}

	return &sharedNode, nil
}

func (s *shareServiceImpl) CreateSharedNode(ctx context.Context, dto *dto.SharedNodeDto) error {
	sharedNode := &vo.SharedNodeVo{
		ID:          dto.ID,
		UserId:      dto.UserId,
		NodeId:      dto.NodeId,
		OwnerId:     dto.OwnerId,
		Description: dto.Description,
		GrantedAt:   dto.GrantedAt,
		RevokedAt:   dto.RevokedAt,
	}

	if err := s.Create(sharedNode).Error; err != nil {
		return err
	}
	return nil
}

func (s *shareServiceImpl) UpdateSharedNode(ctx context.Context, dto *dto.SharedNodeDto) error {
	sharedNode := &vo.SharedNodeVo{
		ID:          dto.ID,
		UserId:      dto.UserId,
		NodeId:      dto.NodeId,
		OwnerId:     dto.OwnerId,
		Description: dto.Description,
		GrantedAt:   dto.GrantedAt,
		RevokedAt:   dto.RevokedAt,
	}

	if err := s.Save(sharedNode).Error; err != nil {
		return err
	}
	return nil
}

func (s *shareServiceImpl) DeleteSharedNode(ctx context.Context, inviteId, nodeId uint) error {
	var (
		err error
	)
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err = s.Model(&entity.SharedNode{}).Where("invite_id = ? and node_id = ?", inviteId, nodeId).Delete(&entity.SharedNode{}).Error; err != nil {
			return err
		}

		if err = s.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ? and resource_id  = ?", inviteId, nodeId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
			return err
		}

		return nil
	})
}

func (s *shareServiceImpl) GetSharedLabel(ctx context.Context, id string) (*vo.SharedLabelVo, error) {
	var (
		sharedLabel vo.SharedLabelVo
		err         error
	)
	if err = s.Model(&sharedLabel).Where("id = ?", id).First(&sharedLabel).Error; err != nil {
		return nil, err
	}

	return &sharedLabel, nil
}

func (s *shareServiceImpl) CreateSharedLabel(ctx context.Context, dto *dto.SharedLabelDto) error {
	sharedLabel := &vo.SharedLabelVo{
		ID:          dto.ID,
		UserId:      dto.UserId,
		LabelId:     dto.LabelId,
		OwnerId:     dto.OwnerId,
		Description: dto.Description,
		GrantedAt:   dto.GrantedAt,
		RevokedAt:   dto.RevokedAt,
	}

	if err := s.Create(sharedLabel).Error; err != nil {
		return err
	}
	return nil
}

func (s *shareServiceImpl) UpdateSharedLabel(ctx context.Context, dto *dto.SharedLabelDto) error {
	sharedLabel := &vo.SharedLabelVo{
		ID:          dto.ID,
		UserId:      dto.UserId,
		LabelId:     dto.LabelId,
		OwnerId:     dto.OwnerId,
		Description: dto.Description,
		GrantedAt:   dto.GrantedAt,
		RevokedAt:   dto.RevokedAt,
	}

	if err := s.Save(sharedLabel).Error; err != nil {
		return err
	}
	return nil
}

func (s *shareServiceImpl) DeleteSharedLabel(ctx context.Context, inviteId, labelId uint) error {
	var (
		err error
	)
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err = s.Model(&entity.SharedLabel{}).Where("invite_id = ? and label_id = ?", inviteId, labelId).Delete(&entity.SharedLabel{}).Error; err != nil {
			return err
		}

		if err = s.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ? and resource_id  = ?", inviteId, labelId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
			return err
		}

		return nil
	})
}

func (s *shareServiceImpl) AddNodeToGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	return s.groupService.UpdateGroup(ctx, dto)
}

func (s *shareServiceImpl) AddPolicyToGroup(ctx context.Context, dto *dto.NodeGroupDto) error {
	return s.groupService.UpdateGroup(ctx, dto)
}

func (s *shareServiceImpl) ListGroups(ctx context.Context, params *dto.SharedGroupParams) (*vo.PageVo, error) {
	var (
		err      error
		groups   []entity.SharedNodeGroup
		db       *gorm.DB
		groupVos []*vo.SharedNodeGroupVo
	)

	result := new(vo.PageVo)
	db = s.DB

	sql, wrappers := utils.GenerateSql(params)
	if sql != "" {
		db = s.DB.Where(sql, wrappers...)
	}

	if err = db.Model(&entity.SharedNodeGroup{}).Preload("GroupNodes").Preload("GroupPolicies").Count(&result.Total).Offset(params.Size * (params.Page - 1)).Limit(params.Size).Find(&groups).Error; err != nil {
		return nil, err
	}

	for _, group := range groups {
		sharedGroupVo := &vo.SharedNodeGroupVo{
			GroupRelationVo: nil,
			ModelVo: vo.ModelVo{
				ID: group.ID,
			},
			Name:        group.GroupName,
			NodeCount:   0,
			Status:      group.AcceptStatus.String(),
			Description: group.Description,
		}

		groupRelationVo := vo.NewGroupRelationVo()
		sharedGroupVo.GroupRelationVo = groupRelationVo

		// fill group nodes
		if len(group.GroupNodes) > 0 {
			sharedGroupVo.GroupNodes = make([]vo.GroupNodeVo, 0)
			for _, node := range group.GroupNodes {
				sharedGroupVo.GroupNodes = append(sharedGroupVo.GroupNodes, vo.GroupNodeVo{
					NodeId:   node.NodeId,
					NodeName: node.NodeName,
				})
				sharedGroupVo.NodeCount++
				groupRelationVo.NodeValues[fmt.Sprintf("%d", node.NodeId)] = node.NodeName // for tom-select show, use nodeId as key
			}

		}

		// fill group policies
		if len(group.GroupPolicies) > 0 {
			sharedGroupVo.GroupPolicies = make([]vo.GroupPolicyVo, 0)
			for _, policy := range group.GroupPolicies {
				sharedGroupVo.GroupPolicies = append(sharedGroupVo.GroupPolicies, vo.GroupPolicyVo{
					PolicyId:   policy.PolicyId,
					PolicyName: policy.PolicyName,
				})
				sharedGroupVo.NodeCount++

				groupRelationVo.PolicyValues[fmt.Sprintf("%d", policy.PolicyId)] = policy.PolicyName // for tom-select show, use policyId as key
			}
		}

		groupVos = append(groupVos, sharedGroupVo)

	}

	result.Data = groupVos
	result.Page = params.Page
	result.Size = params.Size

	return result, nil
}

func (s *shareServiceImpl) ListNodes(ctx context.Context, params *dto.SharedNodeParams) (*vo.PageVo, error) {
	var (
		err     error
		nodes   []entity.SharedNode
		db      *gorm.DB
		nodeVos []*vo.SharedNodeVo
	)

	result := new(vo.PageVo)
	db = s.DB

	sql, wrappers := utils.GenerateSql(params)
	if sql != "" {
		db = s.DB.Where(sql, wrappers...)
	}

	if err = db.Model(&entity.SharedNode{}).Preload("Node").Preload("NodeLabels").Count(&result.Total).Offset(params.Size * (params.Page - 1)).Limit(params.Size).Find(&nodes).Error; err != nil {
		return nil, err
	}

	for _, node := range nodes {
		sharedNodeVo := &vo.SharedNodeVo{
			ID:          node.ID,
			UserId:      node.UserId,
			NodeId:      node.NodeId,
			AppId:       node.Node.AppID,
			Address:     node.Node.Address,
			Name:        node.Node.Name,
			OwnerId:     node.OwnerId,
			Description: node.Description,
			GrantedAt:   node.GrantedAt.Time,
			RevokedAt:   node.RevokedAt.Time,
		}

		labelResrouceVo := vo.NewLabelResourceVo()
		sharedNodeVo.LabelResourceVo = labelResrouceVo

		if len(node.NodeLabels) > 0 {
			sharedNodeVo.NodeLabels = make([]vo.NodeLabelVo, 0)
			for _, label := range node.NodeLabels {
				sharedNodeVo.NodeLabels = append(sharedNodeVo.NodeLabels, vo.NodeLabelVo{
					LabelId:   label.LabelId,
					LabelName: label.LabelName,
				})

				labelResrouceVo.LabelValues[fmt.Sprintf("%d", label.LabelId)] = label.LabelName // for tom-select show, use policyId as key
			}
		}

		nodeVos = append(nodeVos, sharedNodeVo)
	}

	result.Data = nodeVos
	result.Page = params.Page
	result.Size = params.Size

	return result, nil
}
