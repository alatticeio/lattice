package service

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type SharedService interface {

	// Shared Group
	GetSharedGroup(ctx context.Context, id string) (*vo.SharedGroupVo, error)
	ListSharedGroup(ctx context.Context, userId string) ([]*vo.SharedGroupVo, error)
	CreateSharedGroup(ctx context.Context, dto *dto.SharedGroupDto) error
	UpdateSharedGroup(ctx context.Context, dto *dto.SharedGroupDto) error
	DeleteSharedGroup(ctx context.Context, inviteId, groupId uint) error

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
}

func NewSharedService(db *DatabaseService) SharedService {
	return &shareServiceImpl{db, log.NewLogger(log.Loglevel, fmt.Sprintf("[%s ]", "SharedService"))}
}

func (s *shareServiceImpl) GetSharedGroup(ctx context.Context, id string) (*vo.SharedGroupVo, error) {
	var (
		sharedGroup vo.SharedGroupVo
		err         error
	)
	if err = s.Model(&sharedGroup).Where("id = ?", id).First(&sharedGroup).Error; err != nil {
		return nil, err
	}

	return &sharedGroup, nil
}

func (s *shareServiceImpl) ListSharedGroup(ctx context.Context, userId string) ([]*vo.SharedGroupVo, error) {
	var (
		sharedGroups []*vo.SharedGroupVo
		err          error
	)
	if err = s.Model(&sharedGroups).Where("user_id = ?", userId).Find(&sharedGroups).Error; err != nil {
		return nil, err
	}

	return sharedGroups, nil
}

func (s *shareServiceImpl) CreateSharedGroup(ctx context.Context, dto *dto.SharedGroupDto) error {
	sharedGroup := &vo.SharedGroupVo{
		ID:          dto.ID,
		UserId:      dto.UserId,
		GroupId:     dto.GroupId,
		OwnerID:     dto.OwnerID,
		Description: dto.Description,
		GrantedAt:   dto.GrantedAt,
		RevokedAt:   dto.RevokedAt,
	}

	if err := s.Create(sharedGroup).Error; err != nil {
		return err
	}
	return nil
}

func (s *shareServiceImpl) UpdateSharedGroup(ctx context.Context, dto *dto.SharedGroupDto) error {
	sharedGroup := &vo.SharedGroupVo{
		ID:          dto.ID,
		UserId:      dto.UserId,
		GroupId:     dto.GroupId,
		OwnerID:     dto.OwnerID,
		Description: dto.Description,
		GrantedAt:   dto.GrantedAt,
		RevokedAt:   dto.RevokedAt,
	}

	if err := s.Save(sharedGroup).Error; err != nil {
		return err
	}
	return nil
}

func (s *shareServiceImpl) DeleteSharedGroup(ctx context.Context, inviteId, labelId uint) error {
	var (
		sharedGroup vo.SharedGroupVo
		err         error
	)
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err = s.Model(&sharedGroup).Where("invite_id = ? and label_id = ?", inviteId, labelId).Delete(&sharedGroup).Error; err != nil {
			return err
		}

		if err = s.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ? and resource_id  = ?", inviteId, labelId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
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

	if err = s.Model(&sharedPolicy).Where("id = ?", id).First(&sharedPolicy).Error; err != nil {
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
		sharedPolicy vo.SharedPolicyVo
		err          error
	)

	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err = s.Model(&sharedPolicy).Where("invite_id and policy_id = ?", inviteId, policyId).Delete(&sharedPolicy).Error; err != nil {
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
		sharedNode vo.SharedNodeVo
		err        error
	)
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err = s.Model(&sharedNode).Where("invite_id = ? and node_id = ?", inviteId, nodeId).Delete(&sharedNode).Error; err != nil {
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
		sharedGroup vo.SharedGroupVo
		err         error
	)
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err = s.Model(&sharedGroup).Where("invite_id = ? and label_id = ?", inviteId, labelId).Delete(&sharedGroup).Error; err != nil {
			return err
		}

		if err = s.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ? and resource_id  = ?", inviteId, labelId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
			return err
		}

		return nil
	})
}
