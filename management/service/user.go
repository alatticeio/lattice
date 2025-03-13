package service

import (
	"context"
	"errors"
	"github.com/pion/turn/v4"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/linkerrors"
	"linkany/pkg/redis"
	"strconv"
	"strings"
	"time"
)

// UserService is an interface for user mapper
type UserService interface {
	Login(u *dto.UserDto) (*entity.Token, error)
	Register(e *dto.UserDto) (*entity.User, error)

	//Get returns a user by token
	Get(token string) (*entity.User, error)

	GetByUsername(username string) (*entity.User, error)

	//Invite a user join network
	// Invite a user join network
	Invite(dto *dto.InviteDto) error
	DeleteInvite(id string) error
	GetInvitation(userId, email string) (*entity.Invitation, error)
	UpdateInvitation(dto *dto.InvitationDto) error

	//ListInvitations list user invite from others
	ListInvitations(params *dto.InvitationParams) (*vo.PageVo, error)

	//listInvites user invite others list
	ListInvites(params *dto.InvitationParams) (*vo.PageVo, error)

	// User Permit
	//UserPermission grants a user permission to access a resource
	Permit(userID uint, resource string, accessLevel string) error

	//GetPermit fetches the permission details for a specific user and resource
	GetPermit(userID string, resource string) (*entity.UserPermission, error)

	//RevokePermit removes a user's permission to access a resource
	RevokePermit(userID string, resource string) error

	//ListPermits lists all permissions for a specific user
	ListPermits(userID string) ([]*entity.UserPermission, error)
}

var (
	_ UserService = (*userServiceImpl)(nil)
)

type userServiceImpl struct {
	*DatabaseService
	tokener *TokenService
	rdb     *redis.Client
}

func NewUserService(db *DatabaseService, rdb *redis.Client) UserService {
	return &userServiceImpl{DatabaseService: db, tokener: NewTokenService(dataBaseService), rdb: rdb}
}

// Login checks if the user exists and returns a token
func (u *userServiceImpl) Login(dto *dto.UserDto) (*entity.Token, error) {

	var user entity.User
	if err := u.Where("username = ?", dto.Username).First(&user).Error; err != nil {
		return nil, linkerrors.ErrUserNotFound
	}

	if err := utils.ComparePassword(user.Password, dto.Password); err != nil {
		return nil, linkerrors.ErrInvalidPassword
	}

	token, err := u.tokener.Generate(user.Username, user.Password)
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
func (u *userServiceImpl) Register(dto *dto.UserDto) (*entity.User, error) {
	hashedPassword, err := utils.EncryptPassword(dto.Password)
	if err != nil {
		return nil, err
	}
	e := &entity.User{
		Username: dto.Username,
		Password: hashedPassword,
	}
	err = u.Create(e).Error
	if err != nil {
		return nil, err
	}
	return e, nil
}

// Get returns a user by username
func (u *userServiceImpl) Get(token string) (*entity.User, error) {
	userToken, err := u.tokener.Parse(token)
	if err != nil {
		return nil, err
	}

	var user entity.User
	if err := u.Where("username = ?", userToken.Username).Find(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *userServiceImpl) GetByUsername(username string) (*entity.User, error) {
	var user entity.User
	if err := u.Where("username = ?", username).Find(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Invitation
func (u *userServiceImpl) Invite(dto *dto.InviteDto) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		var inviteUser, invitationUser *entity.User
		if inviteUser, err = u.GetByUsername(dto.Username); err != nil {
			return err
		}
		if invitationUser, err = u.GetByUsername(dto.InviteUsername); err != nil {
			return err
		}

		groupName := getGroupNames(tx, dto.GroupIdList)
		invite := &entity.Invites{
			InvitationId: int64(invitationUser.ID),
			InviterId:    int64(inviteUser.ID),
			MobilePhone:  dto.MobilePhone,
			Email:        dto.Email,
			Group:        groupName,
			Permissions:  dto.Permissions,
			AcceptStatus: entity.NewInvite,
			InvitedAt:    time.Now(),
		}
		if err = tx.Create(invite).Error; err != nil {
			return err
		}

		if err = tx.Create(&entity.Invitation{
			InvitationId: invitationUser.ID,
			InviteeId:    inviteUser.ID,
			InviteId:     invite.ID,
			AcceptStatus: entity.NewInvite,
			Permissions:  dto.Permissions,
			Group:        groupName,
			Network:      dto.Network,
		}).Error; err != nil {
			return err
		}

		// insert into user granted permissions
		return addResourcePermission(tx, dto)
	})

}

func addResourcePermission(tx *gorm.DB, dto *dto.InviteDto) error {

	if dto.PolicyIdList != nil {
		names, ids, err := getActualPermission(tx, utils.Policy, dto)
		if err != nil {
			return err
		}

		for _, policyId := range dto.PolicyIdList {
			permit := &entity.UserResourceGrantedPermission{
				OwnerId:       uint(dto.InvitationId),
				InvitedId:     uint(dto.InviterId),
				ResourceType:  utils.Policy,
				ResourceId:    policyId,
				Permission:    utils.Join(names, ","),
				PermissionIds: utils.Join(ids, ","),
			}
			if err := tx.Create(permit).Error; err != nil {
				return err
			}
		}
	}

	if dto.NodeIdList != nil {
		names, ids, err := getActualPermission(tx, utils.Node, dto)
		if err != nil {
			return err
		}

		for _, nodeId := range dto.NodeIdList {
			permit := &entity.UserResourceGrantedPermission{
				OwnerId:       uint(dto.InvitationId),
				InvitedId:     uint(dto.InviterId),
				ResourceType:  utils.Node,
				ResourceId:    nodeId,
				Permission:    utils.Join(names, ","),
				PermissionIds: utils.Join(ids, ","),
			}
			if err := tx.Create(permit).Error; err != nil {
				return err
			}
		}
	}

	if dto.LabelIdList != nil {
		names, ids, err := getActualPermission(tx, utils.Label, dto)
		if err != nil {
			return err
		}

		for _, labelId := range dto.LabelIdList {
			permit := &entity.UserResourceGrantedPermission{
				OwnerId:       uint(dto.InvitationId),
				InvitedId:     uint(dto.InviterId),
				ResourceType:  utils.Label,
				ResourceId:    labelId,
				Permission:    utils.Join(names, ","),
				PermissionIds: utils.Join(ids, ","),
			}
			if err := tx.Create(permit).Error; err != nil {
				return err
			}
		}
	}

	if dto.GroupIdList != nil {
		names, ids, err := getActualPermission(tx, utils.Group, dto)
		if err != nil {
			return err
		}

		for _, groupId := range dto.GroupIdList {
			permit := &entity.UserResourceGrantedPermission{
				OwnerId:       uint(dto.InvitationId),
				InvitedId:     uint(dto.InviterId),
				ResourceType:  utils.Group,
				ResourceId:    groupId,
				Permission:    utils.Join(names, ","),
				PermissionIds: utils.Join(ids, ","),
			}
			if err := tx.Create(permit).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func getActualPermission(tx *gorm.DB, resType utils.ResourceType, dto *dto.InviteDto) ([]string, []uint, error) {
	var permissions []entity.Permissions
	if err := tx.Model(&entity.Permissions{}).Where("id in ? and resource_type = ?", dto.PermissionIdList, resType).Find(&permissions).Error; err != nil {
		return nil, nil, err
	}

	var names []string
	var ids []uint
	for _, permission := range permissions {
		names = append(names, permission.Name)
		ids = append(ids, permission.ID)
	}

	return names, ids, nil
}

func (u *userServiceImpl) DeleteInvite(id string) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		//delete role &  permissions

		var invite entity.Invites

		var err error
		if err = tx.Model(&entity.Invites{}).Where("id = ?", id).Find(&invite).Delete(&entity.Invites{}).Error; err != nil {
			return err
		}

		if err = tx.Model(&entity.Invitation{}).Where("invite_id = ?", invite.ID).Delete(&entity.Invitation{}).Error; err != nil {
			return err
		}

		return nil
	})
}

func getGroupNames(tx *gorm.DB, ids []string) string {
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

func (u *userServiceImpl) GetInvitation(userId, email string) (*entity.Invitation, error) {
	var inv entity.Invitation
	if err := u.Where("invitation_id = ? AND email = ?", userId, email).First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

func (u *userServiceImpl) UpdateInvitation(dto *dto.InvitationDto) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		var (
			inv entity.Invitation
			err error
		)
		if err = tx.Model(&entity.Invitation{}).Where("id = ?", dto.ID).Find(&inv).Update("status = ?", dto.AcceptStatus).Error; err != nil {
			return err
		}

		// if reject, return
		if dto.AcceptStatus == entity.Rejected {
			return nil
		}
		// data insert to shared
		groupIds := strings.Split(inv.Group, ",")
		for _, groupId := range groupIds {
			gid, err := strconv.Atoi(groupId)
			if err != nil {
				return errors.New("invalid groupId")
			}
			shareGroup := &entity.UserGroupShared{
				OwnerId:     inv.InviteeId,
				UserId:      inv.InvitationId,
				GroupId:     uint(gid),
				Description: "",
			}

			if err = tx.Model(&entity.UserGroupShared{}).Create(shareGroup).Error; err != nil {
				return err
			}
		}

		// data insert to permissions
		permissions := strings.Split(inv.Permissions, ",")
		for _, permission := range permissions {
			permit := &entity.UserResourceGrantedPermission{
				InvitedId:    inv.InviteeId,
				OwnerId:      inv.InvitationId,
				ResourceType: 1,
				ResourceId:   "",
				Permission:   permission,
			}
			if err = tx.Model(&entity.UserResourceGrantedPermission{}).Create(permit).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (u *userServiceImpl) ListInvites(params *dto.InvitationParams) (*vo.PageVo, error) {

	var invs []*entity.Invites
	result := new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := u.DB
	if sql != "" {
		db = u.Model(&entity.Invites{}).Where(sql, wrappers)
	}

	if err := db.Model(&entity.Invites{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := db.Model(&entity.Invites{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&invs).Error; err != nil {
		return nil, err
	}

	var insVos []*vo.InviteVo
	for _, inv := range invs {
		var inviteUser entity.User
		var invitationUser entity.User
		var err error
		if err = db.Model(&entity.User{}).Where("id = ?", inv.InviterId).First(&inviteUser).Error; err != nil {
			return nil, err
		}

		if err = db.Model(&entity.User{}).Where("id = ?", inv.InvitationId).First(&invitationUser).Error; err != nil {
			return nil, err
		}

		insVo := &vo.InviteVo{
			ID:           uint64(inv.ID),
			InviteeName:  inviteUser.Username,
			InviterName:  invitationUser.Username,
			MobilePhone:  invitationUser.Mobile,
			Email:        invitationUser.Email,
			Avatar:       invitationUser.Avatar,
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
	return result, nil
}

func (u *userServiceImpl) ListInvitations(params *dto.InvitationParams) (*vo.PageVo, error) {
	var invs []*entity.Invitation
	result := new(vo.PageVo)
	db := u.DB
	sql, wrappers := utils.Generate(params)
	if sql != "" {
		db = u.Model(&entity.Invitation{}).Where(sql, wrappers)
	}

	if err := db.Model(&entity.Invitation{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := u.Model(&entity.Invitation{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&invs).Error; err != nil {
		return nil, err
	}

	var insVos []*vo.InvitationVo
	for _, inv := range invs {
		var inviteUser entity.User
		var err error
		if err = db.Model(&entity.User{}).Where("id = ?", inv.InviteeId).First(&inviteUser).Error; err != nil {
			return nil, err
		}

		insVo := &vo.InvitationVo{
			ID:            uint64(inv.ID),
			Group:         inv.Group,
			InviterName:   inviteUser.Username,
			InviterAvatar: inviteUser.Avatar,
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

	return result, nil
}

func (u *userServiceImpl) Permit(userID uint, resource string, permission string) error {
	//TODO Get user's permissions first, if nil, add, else update

	permit := entity.UserPermission{
		UserID:       userID,
		ResourceType: resource,
		Permissions:  permission,
	}
	if err := u.Create(&permit).Error; err != nil {
		return err
	}
	return nil
}

func (u *userServiceImpl) GetPermit(userID string, resource string) (*entity.UserPermission, error) {
	var permit entity.UserPermission
	if err := u.Where("user_id = ? AND resource = ?", userID, resource).First(&permit).Error; err != nil {
		return nil, err
	}
	return &permit, nil
}

func (u *userServiceImpl) RevokePermit(userID string, resource string) error {
	if err := u.Where("user_id = ? AND resource = ?", userID, resource).Delete(&entity.UserPermission{}).Error; err != nil {
		return err
	}
	return nil
}

func (u *userServiceImpl) ListPermits(userID string) ([]*entity.UserPermission, error) {
	var permits []*entity.UserPermission
	if err := u.Where("user_id = ?", userID).Find(&permits).Error; err != nil {
		return nil, err
	}
	return permits, nil
}
