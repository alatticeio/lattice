package controller

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/management/vo"
	"linkany/pkg/log"
	"linkany/pkg/redis"
)

type UserController struct {
	logger      *log.Logger
	userService service.UserService
}

func NewUserController(db *service.DatabaseService, rdb *redis.Client) *UserController {
	return &UserController{
		userService: service.NewUserService(db, rdb),
		logger:      log.NewLogger(log.Loglevel, "user-controller")}
}

// User module
func (u *UserController) Login(dto *dto.UserDto) (*entity.Token, error) {
	return u.userService.Login(dto)
}

func (u *UserController) Register(e *dto.UserDto) (*entity.User, error) {
	return u.userService.Register(e)
}

func (u *UserController) Get(token string) (*entity.User, error) {
	return u.userService.Get(token)
}

func (u *UserController) QueryUsers(params *dto.UserParams) ([]*vo.UserVo, error) {
	return u.userService.QueryUsers(params)
}

// Invite module
func (u *UserController) Invite(dto *dto.InviteDto) error {
	return u.userService.Invite(dto)
}

//func (u *UserController) GetInvite(ctx context.Context, id string) (*vo.InviteVo, error) {
//	return u.userService.GetInvite(ctx, id)
//}

func (u *UserController) UpdateInvite(ctx context.Context, dto *dto.InviteDto) error {
	return u.userService.UpdateInvite(ctx, dto)
}

func (u *UserController) CancelInvite(id string) error {
	return u.userService.CancelInvite(id)
}

func (u *UserController) DeleteInvite(id string) error {
	return u.userService.DeleteInvite(id)
}

func (u *UserController) RejectInvitation(id uint) error {
	return u.userService.RejectInvitation(id)
}

func (u *UserController) AcceptInvitation(id uint) error {
	return u.userService.AcceptInvitation(id)
}

func (u *UserController) GetInvitation(userId, email string) (*entity.InvitationEntity, error) {
	return u.userService.GetInvitation(userId, email)
}

func (u *UserController) UpdateInvitation(dto *dto.InvitationDto) error {
	return u.userService.UpdateInvitation(dto)
}

func (u *UserController) ListUserInvites(ctx context.Context, params *dto.InvitationParams) (*vo.PageVo, error) {
	return u.userService.ListInvitesEntity(ctx, params)
}

func (u *UserController) ListUserInvitations(ctx context.Context, params *dto.InvitationParams) (*vo.PageVo, error) {
	return u.userService.ListInvitations(ctx, params)
}

// Permit module
func (u *UserController) Permit(userID uint, resource string, accessLevel string) error {
	return u.userService.Permit(userID, resource, accessLevel)
}

func (u *UserController) GetPermit(userID string, resource string) (*entity.UserPermission, error) {
	return u.userService.GetPermit(userID, resource)
}

func (u *UserController) RevokePermit(userID string, resource string) error {
	return u.userService.RevokePermit(userID, resource)
}

func (u *UserController) ListPermits(userID string) ([]*entity.UserPermission, error) {
	return u.userService.ListPermits(userID)
}
