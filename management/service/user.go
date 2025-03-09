package service

import (
	"context"
	"github.com/pion/turn/v4"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/linkerrors"
	"linkany/pkg/redis"
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
	GetInvitation(userId, email string) (*entity.Invitation, error)
	UpdateInvitation(dto *dto.InviteDto) error

	//ListInvitations list user invite from others
	ListInvitations(params *dto.InvitationParams) (*vo.PageVo, error)

	//listInvites user invite others list
	ListInvites(params *dto.InvitationParams) (*vo.PageVo, error)

	// User Permit
	//Permission grants a user permission to access a resource
	Permit(userID uint, resource string, accessLevel string) error

	//GetPermit fetches the permission details for a specific user and resource
	GetPermit(userID string, resource string) (*entity.Permission, error)

	//RevokePermit removes a user's permission to access a resource
	RevokePermit(userID string, resource string) error

	//ListPermits lists all permissions for a specific user
	ListPermits(userID string) ([]*entity.Permission, error)
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

	tx := u.Begin()
	var err error
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var inviteUser, invitationUser *entity.User
	if inviteUser, err = u.GetByUsername(dto.Username); err != nil {
		return err
	}
	if invitationUser, err = u.GetByUsername(dto.InviteUsername); err != nil {
		return err
	}

	if err = tx.Create(&entity.Invites{
		InvitationId: int64(invitationUser.ID),
		InviterId:    int64(inviteUser.ID),
		MobilePhone:  dto.MobilePhone,
		Email:        dto.Email,
		Group:        dto.Group,
		Permissions:  dto.Permissions,
		AcceptStatus: entity.NewInvite,
		InvitedAt:    time.Now(),
	}).Error; err != nil {
		return err
	}

	if err = tx.Create(&entity.Invitation{
		InvitationId: int64(invitationUser.ID),
		InviterId:    int64(inviteUser.ID),
		AcceptStatus: entity.NewInvite,
		Permissions:  dto.Permissions,
		Group:        dto.Group,
		Network:      dto.Network,
	}).Error; err != nil {
		return err
	}

	return nil
}

func (u *userServiceImpl) GetInvitation(userId, email string) (*entity.Invitation, error) {
	var inv entity.Invitation
	if err := u.Where("invitation_id = ? AND email = ?", userId, email).First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

func (u *userServiceImpl) UpdateInvitation(dto *dto.InviteDto) error {
	var inv entity.Invitation
	if err := u.Where("invitation_id = ? AND email = ?", dto.InvitationId, dto.Email).First(&inv).Error; err != nil {
		return err
	}
	inv.AcceptStatus = entity.Accept
	u.Save(&inv)
	return nil
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
		if err = db.Model(&entity.User{}).Where("id = ?", inv.InviterId).First(&inviteUser).Error; err != nil {
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

	permit := entity.Permission{
		UserID:       userID,
		ResourceType: resource,
		Permissions:  permission,
	}
	if err := u.Create(&permit).Error; err != nil {
		return err
	}
	return nil
}

func (u *userServiceImpl) GetPermit(userID string, resource string) (*entity.Permission, error) {
	var permit entity.Permission
	if err := u.Where("user_id = ? AND resource = ?", userID, resource).First(&permit).Error; err != nil {
		return nil, err
	}
	return &permit, nil
}

func (u *userServiceImpl) RevokePermit(userID string, resource string) error {
	if err := u.Where("user_id = ? AND resource = ?", userID, resource).Delete(&entity.Permission{}).Error; err != nil {
		return err
	}
	return nil
}

func (u *userServiceImpl) ListPermits(userID string) ([]*entity.Permission, error) {
	var permits []*entity.Permission
	if err := u.Where("user_id = ?", userID).Find(&permits).Error; err != nil {
		return nil, err
	}
	return permits, nil
}
