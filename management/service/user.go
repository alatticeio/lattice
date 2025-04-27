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
	"linkany/pkg/linkerrors"
	"linkany/pkg/log"
	"linkany/pkg/redis"
	"strconv"
	"strings"
	"time"

	"github.com/pion/turn/v4"
	"gorm.io/gorm"
)

// UserService is an interface for user mapper
type UserService interface {
	Login(ctx context.Context, u *dto.UserDto) (*entity.Token, error)
	Register(ctx context.Context, e *dto.UserDto) (*entity.User, error)

	//Get returns a user by token
	Get(ctx context.Context, token string) (*entity.User, error)

	GetByUsername(ctx context.Context, usernames []string) (map[string]entity.User, error)
	QueryUsers(ctx context.Context, params *dto.UserParams) ([]*vo.UserVo, error)

	//InviterEntity a user join network
	Invite(ctx context.Context, dto *dto.InviteDto) error
	CancelInvite(ctx context.Context, id uint64) error
	DeleteInvite(ctx context.Context, id uint64) error
	UpdateInvite(ctx context.Context, dto *dto.InviteDto) error
	GetInvite(ctx context.Context, id uint64) (*vo.InviteVo, error)
	GetInvitation(ctx context.Context, userId uint64, email string) (*entity.InviteeEntity, error)
	UpdateInvitation(ctx context.Context, dto *dto.InvitationDto) error
	RejectInvitation(ctx context.Context, id uint64) error
	AcceptInvitation(ctx context.Context, id uint64) error

	//ListInvitations list user invite from others
	ListInvitations(ctx context.Context, params *dto.InvitationParams) (*vo.PageVo, error)

	//listInvites user invite others list
	ListInvitesEntity(ctx context.Context, params *dto.InvitationParams) (*vo.PageVo, error)

	// User Permit
	//UserPermission grants a user permission to access a resource
	Permit(ctx context.Context, userID uint64, resource string, accessLevel string) error

	//GetPermit fetches the permission details for a specific user and resource
	GetPermit(ctx context.Context, userID string, resource string) (*entity.UserPermission, error)

	//RevokePermit removes a user's permission to access a resource
	RevokePermit(ctx context.Context, userID string, resource string) error

	//ListPermits lists all permissions for a specific user
	ListPermits(ctx context.Context, userID string) ([]*entity.UserPermission, error)
}

var (
	_ UserService = (*userServiceImpl)(nil)
)

type userServiceImpl struct {
	db           *gorm.DB
	tokenService TokenService
	rdb          *redis.Client
	userRepo     repository.UserRepository
	inviteRepo   repository.InviteRepository
	shareRepo    repository.SharedRepository
	logger       *log.Logger
}

func (u *userServiceImpl) GetInvite(ctx context.Context, id uint64) (*vo.InviteVo, error) {
	//TODO implement me
	panic("implement me")
}

func NewUserService(db *gorm.DB, rdb *redis.Client) UserService {
	return &userServiceImpl{db: db, tokenService: NewTokenService(db), rdb: rdb,
		userRepo:   repository.NewUserRepository(db),
		inviteRepo: repository.NewInviteRepository(db),
		shareRepo:  repository.NewSharedRepository(db),
		logger:     log.NewLogger(log.Loglevel, "user-service")}
}

// Login checks if the user exists and returns a token
func (u *userServiceImpl) Login(ctx context.Context, dto *dto.UserDto) (*entity.Token, error) {

	user, err := u.userRepo.GetByUsername(ctx, dto.Username)
	if err != nil {
		return nil, err
	}

	if err := utils.ComparePassword(user.Password, dto.Password); err != nil {
		return nil, linkerrors.ErrInvalidPassword
	}

	token, err := u.tokenService.Generate(user.Username, user.Password)
	if err != nil {
		return nil, err
	}

	// Save turn key to redis
	key := turn.GenerateAuthKey(user.Username, "linkany.io", dto.Password)
	if err = u.rdb.Set(context.Background(), user.Username, string(key)); err != nil {
		return nil, err
	}
	return &entity.Token{Token: token, Avatar: user.Avatar, Email: user.Email, Mobile: user.Mobile}, nil
}

// Register creates a new user
func (u *userServiceImpl) Register(ctx context.Context, dto *dto.UserDto) (*entity.User, error) {
	hashedPassword, err := utils.EncryptPassword(dto.Password)
	if err != nil {
		return nil, err
	}
	e := &entity.User{
		Username: dto.Username,
		Password: hashedPassword,
	}
	if err = u.userRepo.Create(ctx, e); err != nil {
		return nil, err
	}

	return e, nil
}

// Get returns a user by username
func (u *userServiceImpl) Get(ctx context.Context, token string) (*entity.User, error) {
	userToken, err := u.tokenService.Parse(token)
	if err != nil {
		return nil, err
	}

	return u.userRepo.GetByUsername(ctx, userToken.Username)
}

func (u *userServiceImpl) GetByUsername(ctx context.Context, usernames []string) (map[string]entity.User, error) {
	m := make(map[string]entity.User, 1)
	var (
		err   error
		users []*entity.User
	)
	if users, err = u.userRepo.GetByUsernames(ctx, usernames); err != nil {
		return nil, err
	}

	u.logger.Verbosef("GetByUsername: %v", users)

	return m, nil
}

func (u *userServiceImpl) QueryUsers(ctx context.Context, params *dto.UserParams) ([]*vo.UserVo, error) {
	var (
		users  []*entity.User
		err    error
		result = new(vo.PageVo)
	)

	if users, err = u.userRepo.Query(ctx, params); err != nil {
		return nil, err
	}

	var userVos []*vo.UserVo
	for _, user := range users {
		userVos = append(userVos, &vo.UserVo{
			ID:          user.ID,
			Username:    user.Username,
			Email:       user.Email,
			Avatar:      user.Avatar,
			MobilePhone: user.Mobile,
		})
	}

	result.Data = userVos
	return userVos, nil
}

// InviteeEntity
func (u *userServiceImpl) Invite(ctx context.Context, dto *dto.InviteDto) error {
	return u.db.Transaction(func(tx *gorm.DB) error {
		var err error
		var m map[string]entity.User
		if m, err = u.GetByUsername(ctx, []string{dto.InviterName, dto.InviteeName}); err != nil {
			return err
		}

		inviterUser := m[dto.InviterName]
		inviteeUser := m[dto.InviteeName]

		// first query, if the invitation exists
		inviteRepo := u.inviteRepo.WithTx(tx)
		exists, err := inviteRepo.GetByName(ctx, dto.InviteeName)
		if err != nil {
			return err
		}
		if exists != nil {
			return linkerrors.ErrInvitationExists
		}

		groupName := getGroupNames(tx, dto.GroupIdList)
		invite := &entity.InviterEntity{
			InviteeId:    inviteeUser.ID,
			InviterId:    inviterUser.ID,
			Group:        groupName,
			Permissions:  dto.Permissions,
			AcceptStatus: entity.NewInvite,
			InvitedAt:    time.Now(),
		}
		if err = inviteRepo.CreateInviter(ctx, invite); err != nil {
			return err
		}

		if err = inviteRepo.CreateInvitee(ctx, &entity.InviteeEntity{
			InviteeId:    inviteeUser.ID,
			InviterId:    inviterUser.ID,
			InviteId:     invite.ID,
			AcceptStatus: entity.NewInvite,
			Permissions:  dto.Permissions,
			Group:        groupName,
			Network:      dto.Network,
		}); err != nil {
			return err
		}

		// insert into user granted permissions
		return addResourcePermission(tx, invite.ID, dto)
	})

}

func (u *userServiceImpl) UpdateInvite(ctx context.Context, dto *dto.InviteDto) error {
	return u.db.Transaction(func(tx *gorm.DB) error {
		var err error
		groupName := getGroupNames(tx, dto.GroupIdList)

		if err = tx.Model(&entity.InviterEntity{}).Where("id = ?", dto.ID).Updates(entity.InviterEntity{
			Group:    groupName,
			GroupIds: dto.GroupIds,
		}).Error; err != nil {
			return err
		}

		if err = tx.Model(&entity.InviteeEntity{}).Where("invite_id = ?", dto.ID).Updates(entity.InviteeEntity{
			Group:    groupName,
			GroupIds: dto.GroupIds,
		}).Error; err != nil {
			return err
		}

		// get user from username for invitee
		var invitationUser entity.User
		if err = tx.Model(&entity.User{}).Where("username = ?", dto.InviteeName).First(&invitationUser).Error; err != nil {
			return err
		}

		dto.InvitationId = invitationUser.ID

		// insert into user granted permissions
		return updateResources(tx, dto.ID, dto)
	})
}

func updateResources(tx *gorm.DB, inviteId uint64, dto *dto.InviteDto) error {
	var err error
	if err = tx.Model(&entity.SharedNodeGroup{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedNodeGroup{}).Error; err != nil {
		return err
	}

	if err = tx.Model(&entity.SharedNode{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedNode{}).Error; err != nil {
		return err
	}

	if err = tx.Model(&entity.SharedLabel{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedLabel{}).Error; err != nil {
		return err
	}

	if err = tx.Model(&entity.SharedPolicy{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedPolicy{}).Error; err != nil {
		return err
	}

	if err = tx.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ?", inviteId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
		return err
	}

	return addResourcePermission(tx, inviteId, dto)
}

//func (u *userServiceImpl) GetInvite(ctx context.Context, id string) (*vo.InviteVo, error) {
//	result := new(vo.InviteVo)
//	var invite entity.InviterEntity
//	if err := u.Where("id = ?", id).First(&invite).Error; err != nil {
//		return nil, err
//	}
//
//	var inviteUser, invitationUser entity.User
//	if err := u.Where("id = ?", invite.InviterId).First(&inviteUser).Error; err != nil {
//		return nil, err
//	}
//
//	if err := u.Where("id = ?", invite.InviterId).First(&invitationUser).Error; err != nil {
//		return nil, err
//	}
//	uvo, err := u.genUserResourceVo(invite.ID)
//	if err != nil {
//		return nil, err
//	}
//
//	result.UserResourceVo = uvo
//
//	return result, nil
//}

func addResourcePermission(tx *gorm.DB, inviteId uint64, dto *dto.InviteDto) error {

	var (
		allNames []string
	)

	if dto.PolicyIdList != nil {
		names, values, ids, err := getActualPermission(tx, utils.Policy, dto)
		if err != nil {
			return err
		}
		allNames = append(allNames, names...)

		var policies []entity.Node
		if err = tx.Model(&entity.Node{}).Where("id in ?", dto.GroupIdList).Find(&policies).Error; err != nil {
			return err
		}

		for _, policy := range policies {

			// insert into shared policy
			sharedPolicy := &entity.SharedPolicy{
				OwnerId:      dto.InviteeId,
				UserId:       dto.InvitationId,
				InviteId:     inviteId,
				AcceptStatus: entity.NewInvite,
				PolicyId:     policy.ID,
				PolicyName:   policy.Name,
				GrantedAt:    utils.NewNullTime(time.Now()),
			}

			if err = tx.Model(&entity.SharedPolicy{}).Create(sharedPolicy).Error; err != nil {
				return err
			}

			if err = createResourcePermission(&createPermissonParams{
				Tx:               tx,
				ResourceType:     utils.Policy,
				OwnerId:          sharedPolicy.OwnerId,
				InvitationId:     sharedPolicy.UserId,
				InviteId:         inviteId,
				ResourceId:       sharedPolicy.ID,
				PermissionTexts:  names,
				PermissionValues: values,
				PermissionIds:    ids,
			}); err != nil {
				return err
			}
		}
	}

	if dto.NodeIdList != nil {
		names, values, ids, err := getActualPermission(tx, utils.Node, dto)
		if err != nil {
			return err
		}
		allNames = append(allNames, names...)

		var nodes []entity.Node
		if err = tx.Model(&entity.Node{}).Where("id in ?", dto.NodeIdList).Find(&nodes).Error; err != nil {
			return err
		}

		for _, node := range nodes {
			// insert into shared node
			sharedNode := &entity.SharedNode{
				OwnerId:      dto.InviteeId,
				UserId:       dto.InvitationId,
				InviteId:     inviteId,
				AcceptStatus: entity.NewInvite,
				NodeId:       node.ID,
				NodeName:     node.Name,
				GrantedAt:    utils.NewNullTime(time.Now()),
			}

			if err = tx.Model(&entity.SharedNode{}).Create(sharedNode).Error; err != nil {
				return err
			}

			if err = createResourcePermission(&createPermissonParams{
				Tx:               tx,
				ResourceType:     utils.Node,
				OwnerId:          sharedNode.OwnerId,
				InvitationId:     sharedNode.UserId,
				InviteId:         inviteId,
				ResourceId:       sharedNode.ID,
				PermissionTexts:  names,
				PermissionValues: values,
				PermissionIds:    ids,
			}); err != nil {
				return err
			}
		}
	}

	if dto.LabelIdList != nil {
		names, values, ids, err := getActualPermission(tx, utils.Label, dto)
		if err != nil {
			return err
		}
		allNames = append(allNames, names...)

		var labels []entity.Label
		if err = tx.Model(&entity.Label{}).Where("id in ?", dto.LabelIdList).Find(&labels).Error; err != nil {
			return err
		}

		for _, label := range labels {

			// insert into shared label
			sharedLabel := &entity.SharedLabel{
				OwnerId:      dto.InviteeId,
				UserId:       dto.InvitationId,
				InviteId:     inviteId,
				AcceptStatus: entity.NewInvite,
				LabelId:      label.ID,
				LabelName:    label.Label,
				GrantedAt:    utils.NewNullTime(time.Now()),
			}

			if err = tx.Model(&entity.SharedLabel{}).Create(sharedLabel).Error; err != nil {
				return err
			}

			if err = createResourcePermission(&createPermissonParams{
				Tx:               tx,
				ResourceType:     utils.Label,
				OwnerId:          sharedLabel.OwnerId,
				InvitationId:     sharedLabel.UserId,
				InviteId:         inviteId,
				ResourceId:       sharedLabel.ID,
				PermissionTexts:  names,
				PermissionValues: values,
				PermissionIds:    ids,
			}); err != nil {
				return err
			}
		}
	}

	if dto.GroupIdList != nil {
		names, values, ids, err := getActualPermission(tx, utils.Group, dto)
		if err != nil {
			return err
		}
		allNames = append(allNames, names...)

		var groups []entity.NodeGroup
		if err = tx.Model(&entity.NodeGroup{}).Where("id in ?", dto.GroupIdList).Find(&groups).Error; err != nil {
			return err
		}

		for _, group := range groups {
			// insert into shared group
			sharedGroup := &entity.SharedNodeGroup{
				OwnerId:      dto.InviteeId,
				UserId:       dto.InvitationId,
				GroupName:    group.Name,
				InviteId:     inviteId,
				AcceptStatus: entity.NewInvite,
				GroupId:      group.ID,
				GrantedAt:    utils.NewNullTime(time.Now()),
			}

			if err = tx.Model(&entity.SharedNodeGroup{}).Create(sharedGroup).Error; err != nil {
				return err
			}

			if err = createResourcePermission(&createPermissonParams{
				Tx:               tx,
				ResourceType:     utils.Group,
				OwnerId:          sharedGroup.OwnerId,
				InvitationId:     sharedGroup.UserId,
				InviteId:         inviteId,
				ResourceId:       sharedGroup.GroupId,
				PermissionTexts:  names,
				PermissionValues: values,
				PermissionIds:    ids,
			}); err != nil {
				return err
			}
		}
	}

	// update invite permissinos
	if err := tx.Model(&entity.InviterEntity{}).Where("id = ?", inviteId).Update("permissions", strings.Join(allNames, ",")).Error; err != nil {
		return err
	}

	return nil
}

type createPermissonParams struct {
	Tx               *gorm.DB
	ResourceType     utils.ResourceType
	OwnerId          uint64
	InvitationId     uint64
	InviteId         uint64
	ResourceId       uint64
	PermissionTexts  []string
	PermissionValues []string
	PermissionIds    []uint64
}

func createResourcePermission(params *createPermissonParams) error {
	for i, permissionText := range params.PermissionTexts {
		permit := &entity.UserResourceGrantedPermission{
			OwnerId:         params.OwnerId,
			InvitationId:    params.InvitationId,
			ResourceType:    params.ResourceType,
			ResourceId:      params.ResourceId,
			InviteId:        params.InviteId,
			PermissionText:  permissionText,
			PermissionValue: params.PermissionValues[i],
			PermissionId:    params.PermissionIds[i],
		}
		if err := params.Tx.Create(permit).Error; err != nil {
			return err
		}
	}

	return nil

}

// getActualPermission return names, values, ids, err
func getActualPermission(tx *gorm.DB, resType utils.ResourceType, dto *dto.InviteDto) ([]string, []string, []uint64, error) {
	var permissions []entity.Permissions
	if err := tx.Model(&entity.Permissions{}).Where("id in ? and permission_type = ?", dto.PermissionIdList, resType.String()).Find(&permissions).Error; err != nil {
		return nil, nil, nil, err
	}

	var names []string
	var ids []uint64
	var values []string
	for _, permission := range permissions {
		names = append(names, permission.Name)
		ids = append(ids, permission.ID)
		values = append(values, permission.PermissionValue)
	}

	return names, values, ids, nil
}

func (u *userServiceImpl) CancelInvite(ctx context.Context, id uint64) error {
	return u.db.Transaction(func(tx *gorm.DB) error {
		//delete role &  permissions

		var invite entity.InviterEntity

		var err error
		if err = tx.Model(&entity.InviterEntity{}).Where("id = ?", id).Find(&invite).Update("accept_status", entity.Canceled).Error; err != nil {
			return err
		}

		inviteRepo := u.inviteRepo.WithTx(tx)
		updateEntity := &entity.InviterEntity{
			AcceptStatus: entity.Canceled,
		}
		updateEntity.ID = id
		if err = inviteRepo.UpdateInviter(ctx, updateEntity); err != nil {
			return err
		}

		return updateResourcePermission(ctx, tx, invite.ID, entity.Canceled)

	})
}

func (u *userServiceImpl) DeleteInvite(ctx context.Context, id uint64) error {
	return u.db.Transaction(func(tx *gorm.DB) error {
		//delete role &  permissions

		var (
			invite entity.InviterEntity
			err    error
		)
		inviteRepo := u.inviteRepo.WithTx(tx)
		if err = inviteRepo.DeleteInviter(ctx, id); err != nil {
			return err
		}

		return deleteResourcePermission(tx, invite.ID)

	})
}

func getGroupNames(tx *gorm.DB, ids []uint64) string {
	var result []string
	for _, id := range ids {
		var group entity.NodeGroup
		if err := tx.Where("id = ?", id).First(&group).Error; err != nil {
			return ""
		}
		result = append(result, group.Name)
	}

	return utils.Join(result, ",")
}

func (u *userServiceImpl) GetInvitation(ctx context.Context, userId uint64, email string) (*entity.InviteeEntity, error) {
	return u.inviteRepo.GetByInviteeIdEmail(ctx, userId, email)
}

func (u *userServiceImpl) UpdateInvitation(ctx context.Context, dto *dto.InvitationDto) error {
	return u.db.Transaction(func(tx *gorm.DB) error {
		var (
			inv entity.InviteeEntity
			err error
		)

		inviteRepo := u.inviteRepo.WithTx(tx)

		if err = inviteRepo.UpdateInvitee(ctx, &entity.InviteeEntity{
			Model: entity.Model{
				ID: dto.ID,
			},
			AcceptStatus: dto.AcceptStatus,
		}); err != nil {
			return err
		}

		// if reject, return
		if dto.AcceptStatus == entity.Rejected {
			return nil
		}
		// data insert to shared
		groupIds := strings.Split(inv.GroupIds, ",")

		for _, groupId := range groupIds {
			gid, err := strconv.ParseUint(groupId, 10, 64)
			if err != nil {
				return errors.New("invalid groupId")
			}
			shareGroup := &entity.SharedNodeGroup{
				OwnerId:     inv.InviterId,
				UserId:      inv.InviteeId,
				GroupId:     gid,
				Description: "",
			}

			if err = tx.Model(&entity.SharedNodeGroup{}).Create(shareGroup).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (u *userServiceImpl) RejectInvitation(ctx context.Context, id uint64) error {
	return u.db.Transaction(func(tx *gorm.DB) error {
		var err error
		inviteRepo := u.inviteRepo.WithTx(tx)
		if err = inviteRepo.UpdateInvitee(ctx, &entity.InviteeEntity{
			Model: entity.Model{
				ID: id,
			},
			AcceptStatus: entity.Rejected,
		}); err != nil {
			return err
		}
		return updateResourcePermission(ctx, tx, id, entity.Rejected)
	})
}

func (u *userServiceImpl) AcceptInvitation(ctx context.Context, id uint64) error {
	return u.db.Transaction(func(tx *gorm.DB) error {
		var err error
		inviteRepo := u.inviteRepo.WithTx(tx)
		if err = inviteRepo.UpdateInvitee(ctx, &entity.InviteeEntity{
			Model: entity.Model{
				ID: id,
			},
			AcceptStatus: entity.Accept,
		}); err != nil {
			return err
		}
		// update shared and permissions table
		return updateResourcePermission(ctx, tx, id, entity.Accept)
	})
}

func updateResourcePermission(ctx context.Context, tx *gorm.DB, inviteId uint64, status entity.AcceptStatus) error {
	// update shared group
	var (
		err error
	)
	sharedRepo := repository.NewSharedRepository(tx)

	//if err = tx.Model(&entity.SharedNodeGroup{}).Where("invite_id = ?", inviteId).Update("accept_status", status).Error; err != nil {
	//	return err
	//}
	if err = sharedRepo.UpdateGroups(ctx, &entity.SharedNodeGroup{
		AcceptStatus: status,
	}, &dto.SharedGroupParams{
		InviteId: inviteId,
	}); err != nil {
		return err
	}

	// update shared node
	if err = sharedRepo.UpdateNodes(ctx, &entity.SharedNode{
		AcceptStatus: status,
	}, &dto.SharedNodeParams{
		InviteId: &inviteId,
	}); err != nil {
		return err
	}

	// update shared label
	if err = sharedRepo.UpdateLabels(ctx, &entity.SharedLabel{
		AcceptStatus: status,
	}, &dto.SharedLabelParams{
		InviteId: &inviteId,
	}); err != nil {
		return err
	}

	// update shared policy
	if err = sharedRepo.UpdatePolicies(ctx, &entity.SharedPolicy{
		AcceptStatus: status,
	}, &dto.SharedPolicyParams{
		InviteId: &inviteId,
	}); err != nil {
		return err
	}

	// update shared perissions
	//if err = tx.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ?", inviteId).Update("accept_status", status).Error; err != nil {
	//	return err
	//}
	//TODO
	//if err = sharedRepo.UpdatePermissions(ctx, &entity.SharedNode{
	//	AcceptStatus: status,
	//}, &dto.SharedNodeParams{
	//	InviteId: inviteId,
	//}); err != nil {
	//	return err
	//}

	switch status {
	case entity.Canceled:
		if err = tx.Model(&entity.InviteeEntity{}).Where("invite_id = ?", inviteId).Update("accept_status", entity.Canceled).Error; err != nil {
			return err
		}
	default:
		// update invite table
		if err = tx.Model(&entity.InviterEntity{}).Where("id = ?", inviteId).Update("accept_status", status).Error; err != nil {
			return err
		}
	}

	return nil
}

func deleteResourcePermission(tx *gorm.DB, inviteId uint64) error {
	// update shared group
	var (
		err error
	)
	if err = tx.Model(&entity.SharedNodeGroup{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedNodeGroup{}).Error; err != nil {
		return err
	}

	// update shared node
	if err = tx.Model(&entity.SharedNode{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedNode{}).Error; err != nil {
		return err
	}

	// update shared label
	if err = tx.Model(&entity.SharedLabel{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedLabel{}).Error; err != nil {
		return err
	}

	// update shared policy
	if err = tx.Model(&entity.SharedPolicy{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedPolicy{}).Error; err != nil {
		return err
	}

	// update shared perissions
	if err = tx.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ?", inviteId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
		return err
	}
	return nil
}

func (u *userServiceImpl) ListInvitesEntity(ctx context.Context, params *dto.InvitationParams) (*vo.PageVo, error) {

	var (
		invs   []*entity.InviterEntity
		result = new(vo.PageVo)
		count  int64
		err    error
	)

	if invs, count, err = u.inviteRepo.ListInviters(ctx, &dto.InviterParams{
		InviterId: utils.GetUserIdFromCtx(ctx),
	}); err != nil {
		return nil, err
	}

	var insVos []*vo.InviteVo

	//var inviteIds []uint
	for _, inv := range invs {
		uvo := new(vo.UserResourceVo)

		//group resource vo
		if len(inv.SharedGroups) > 0 {
			groupResourceVo := new(vo.GroupResourceVo)
			uvo.GroupResourceVo = groupResourceVo

			//var groupValues []*vo.ResourceValue
			groupValues := make(map[string]string, 1)
			var groupNames []string
			var groupIds []string
			for _, group := range inv.SharedGroups {
				groupValues[fmt.Sprintf("%d", group.GroupId)] = group.GroupName
				groupNames = append(groupNames, group.GroupName)
				groupIds = append(groupIds, fmt.Sprintf("%d", group.ID))
			}

			inv.Group = strings.Join(groupNames, ",")
			inv.GroupIds = strings.Join(groupIds, ",")

			groupResourceVo.GroupValues = groupValues
		}

		if len(inv.SharedNodes) > 0 {
			// node resource vo
			nodeResourceVo := new(vo.NodeResourceVo)
			uvo.NodeResourceVo = nodeResourceVo

			nodeValues := make(map[string]string, 1)
			for _, node := range inv.SharedNodes {
				nodeValues[fmt.Sprintf("%d", node.NodeId)] = node.NodeName
			}
			nodeResourceVo.NodeValues = nodeValues
		}

		if len(inv.SharedPolicies) > 0 {
			// policy resource vo
			policyResourceVo := new(vo.PolicyResourceVo)
			uvo.PolicyResourceVo = policyResourceVo

			policyValues := make(map[string]string, 1)
			for _, policy := range inv.SharedPolicies {
				policyValues[fmt.Sprintf("%d", policy.PolicyId)] = policy.PolicyName
			}
			policyResourceVo.PolicyValues = policyValues
		}

		if len(inv.SharedLabels) > 0 {
			// label resource vo
			labelResourceVo := new(vo.LabelResourceVo)
			uvo.LabelResourceVo = labelResourceVo

			labelValues := make(map[string]string, 1)
			for _, label := range inv.SharedLabels {
				labelValues[fmt.Sprintf("%d", label.LabelId)] = label.LabelName
			}
			labelResourceVo.LabelValues = labelValues
		}

		if len(inv.SharedPermissions) > 0 {
			permissionResourceVo := new(vo.PermissionResourceVo)
			uvo.PermissionResourceVo = permissionResourceVo
			permissionValues := make(map[string]string, 1)
			var permissionNames []string
			for _, sharedPermission := range inv.SharedPermissions {
				if (permissionValues[fmt.Sprintf("%d", sharedPermission.PermissionId)]) != "" {
					continue
				}
				permissionNames = append(permissionNames, sharedPermission.PermissionText)
				permissionValues[fmt.Sprintf("%d", sharedPermission.PermissionId)] = sharedPermission.PermissionText
			}

			inv.Permissions = strings.Join(permissionNames, ",")

			permissionResourceVo.PermissionValues = permissionValues
		}

		insVo := &vo.InviteVo{
			UserResourceVo: uvo,
			ID:             uint64(inv.ID),
			InviteeName:    inv.InviteeUser.Username,
			//InviteeName:    inv.InvitationUsername,
			InviterName:  inv.InviterUser.Username,
			MobilePhone:  inv.InviteeUser.Mobile,
			Email:        inv.InviteeUser.Email,
			Avatar:       inv.InviteeUser.Avatar,
			Role:         inv.Role,
			GroupName:    inv.Group,
			Permissions:  inv.Permissions,
			AcceptStatus: inv.AcceptStatus.String(),
			InvitedAt:    inv.InvitedAt,
		}

		insVos = append(insVos, insVo)
	}

	result.Data = insVos
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size
	result.Total = count
	return result, nil
}

func (u *userServiceImpl) ListInvitations(ctx context.Context, params *dto.InvitationParams) (*vo.PageVo, error) {
	var (
		invs   []*entity.InviteeEntity
		err    error
		count  int64
		result = new(vo.PageVo)
	)

	if invs, count, err = u.inviteRepo.ListInvitees(ctx, &dto.InvitationParams{}); err != nil {
		return nil, err
	}

	var insVos []*vo.InvitationVo
	for _, inv := range invs {
		insVo := &vo.InvitationVo{
			ID:            uint64(inv.ID),
			Group:         inv.Group,
			InviterName:   inv.User.Username,
			InviterAvatar: inv.User.Avatar,
			InviteId:      inv.InviteId,
			Role:          inv.Role,
			AcceptStatus:  inv.AcceptStatus.String(),
			Permissions:   inv.Permissions,

			InvitedAt: inv.InvitedAt,
		}

		insVos = append(insVos, insVo)
	}

	result.Data = insVos
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size
	result.Total = count

	return result, nil
}

func (u *userServiceImpl) Permit(ctx context.Context, userID uint64, resource string, permission string) error {
	//TODO Get user's permissions first, if nil, add, else update

	return nil
}

func (u *userServiceImpl) GetPermit(ctx context.Context, userID string, resource string) (*entity.UserPermission, error) {
	var permit entity.UserPermission
	return &permit, nil
}

func (u *userServiceImpl) RevokePermit(ctx context.Context, userID string, resource string) error {
	return nil
}

func (u *userServiceImpl) ListPermits(ctx context.Context, userID string) ([]*entity.UserPermission, error) {
	var permits []*entity.UserPermission
	return permits, nil
}
