package service

import (
	"context"
	"github.com/pion/turn/v4"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
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

	//Invite a user join network
	// Invite a user join network
	Invite(dto *dto.InviteDto) error
	GetInvitation(userId, email string) (*entity.Invitation, error)
	UpdateInvitation(dto *dto.InviteDto) error
	ListInvitations(paras *InvitationParams) ([]*entity.Invitation, error)

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
	tokener *utils.Tokener
	rdb     *redis.Client
}

type InviteType string

var (
	INVITE  InviteType = "invite"  // invite to others
	INVITED InviteType = "invited" // other invite to
)

type InvitationParams struct {
	dto.PageModel
	UserId      *string
	Email       *string
	MobilePhone *string
	Type        *InviteType
	Status      *dto.AcceptType
}

func (p *InvitationParams) Generate() []*dto.KeyValue {
	var result []*dto.KeyValue

	if p.UserId != nil {
		result = append(result, dto.NewKV("user_id", p.UserId))
	}

	if p.Type != nil {
		result = append(result, dto.NewKV("Type", p.Type))
	}

	if p.Status != nil {
		result = append(result, dto.NewKV("status", p.Status))
	}

	result = append(result, dto.NewKV("page_no", p.PageNo))
	result = append(result, dto.NewKV("page_size", p.PageSize))

	return result
}

func NewUserService(db *DatabaseService, rdb *redis.Client) UserService {
	return &userServiceImpl{DatabaseService: db, tokener: utils.NewTokener(), rdb: rdb}
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
	return &entity.Token{Token: token}, nil
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

// Invitation
func (u *userServiceImpl) Invite(dto *dto.InviteDto) error {
	return u.Create(&entity.Invitation{
		InvitationId: dto.InvitationId,
		InviterId:    dto.InviterId,
		MobilePhone:  dto.MobilePhone,
		Email:        dto.Email,
		AcceptStatus: entity.NewInvite,
		InvitedAt:    time.Now(),
	}).Error
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
	inv.AcceptAt = time.Now()
	u.Save(&inv)
	return nil
}

func (u *userServiceImpl) ListInvitations(params *InvitationParams) ([]*entity.Invitation, error) {
	var invs []*entity.Invitation
	sql, wrappers := utils.Generate(params)
	if err := u.Where(sql, wrappers).Find(&invs).Error; err != nil {
		return nil, err
	}
	return invs, nil
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
