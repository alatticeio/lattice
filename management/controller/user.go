package controller

import (
	"context"
	"gorm.io/gorm"
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

func NewUserController(db *gorm.DB, rdb *redis.Client) *UserController {
	return &UserController{
		userService: service.NewUserService(db, rdb),
		logger:      log.NewLogger(log.Loglevel, "user-controller")}
}

// User module
func (u *UserController) Login(ctx context.Context, dto *dto.UserDto) (*entity.Token, error) {
	return u.userService.Login(ctx, dto)
}

func (u *UserController) Register(ctx context.Context, e *dto.UserDto) (*entity.User, error) {
	return u.userService.Register(ctx, e)
}

func (u *UserController) Get(ctx context.Context, token string) (*entity.User, error) {
	return u.userService.Get(ctx, token)
}

func (u *UserController) QueryUsers(ctx context.Context, params *dto.UserParams) ([]*vo.UserVo, error) {
	return u.userService.QueryUsers(ctx, params)
}

// Invite module
func (u *UserController) Invite(ctx context.Context, dto *dto.InviteDto) error {
	return u.userService.Invite(ctx, dto)
}

//func (u *UserController) GetInvite(ctx context.Context, id string) (*vo.InviteVo, error) {
//	return u.userService.GetInvite(ctx, id)
//}

func (u *UserController) UpdateInvite(ctx context.Context, dto *dto.InviteDto) error {
	return u.userService.UpdateInvite(ctx, dto)
}

func (u *UserController) CancelInvite(ctx context.Context, id uint64) error {
	return u.userService.CancelInvite(ctx, id)
}

func (u *UserController) DeleteInvite(ctx context.Context, id uint64) error {
	return u.userService.DeleteInvite(ctx, id)
}

func (u *UserController) RejectInvitation(ctx context.Context, id uint64) error {
	return u.userService.RejectInvitation(ctx, id)
}

func (u *UserController) AcceptInvitation(ctx context.Context, id uint64) error {
	return u.userService.AcceptInvitation(ctx, id)
}

func (u *UserController) GetInvitation(ctx context.Context, userId uint64, email string) (*entity.InviteeEntity, error) {
	return u.userService.GetInvitation(ctx, userId, email)
}

func (u *UserController) UpdateInvitation(ctx context.Context, dto *dto.InvitationDto) error {
	return u.userService.UpdateInvitation(ctx, dto)
}

func (u *UserController) ListUserInvites(ctx context.Context, params *dto.InvitationParams) (*vo.PageVo, error) {
	return u.userService.ListInvitesEntity(ctx, params)
}

func (u *UserController) ListUserInvitations(ctx context.Context, params *dto.InvitationParams) (*vo.PageVo, error) {
	return u.userService.ListInvitations(ctx, params)
}

// Permit module
func (u *UserController) Permit(ctx context.Context, userID uint64, resource string, accessLevel string) error {
	return u.userService.Permit(ctx, userID, resource, accessLevel)
}

func (u *UserController) GetPermit(ctx context.Context, userID string, resource string) (*entity.UserPermission, error) {
	return u.userService.GetPermit(ctx, userID, resource)
}

func (u *UserController) RevokePermit(ctx context.Context, userID string, resource string) error {
	return u.userService.RevokePermit(ctx, userID, resource)
}

func (u *UserController) ListPermits(ctx context.Context, userID string) ([]*entity.UserPermission, error) {
	return u.userService.ListPermits(ctx, userID)
}
