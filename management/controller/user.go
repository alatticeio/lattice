package controller

import (
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/pkg/log"
)

type UserController struct {
	logger      *log.Logger
	userService service.UserService
}

func NewUserController(userMapper service.UserService) *UserController {
	return &UserController{userService: userMapper, logger: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "user-controller"))}
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

// Invite module
func (u *UserController) Invite(dto *dto.InviteDto) error {
	return u.userService.Invite(dto)
}

func (u *UserController) GetInvitation(userId, email string) (*entity.Invitation, error) {
	return u.userService.GetInvitation(userId, email)
}

func (u *UserController) UpdateInvitation(dto *dto.InviteDto) error {
	return u.userService.UpdateInvitation(dto)
}

func (u *UserController) ListInvitations() ([]*entity.Invitation, error) {
	return u.userService.ListInvitations(&service.InvitationParams{})
}

// Permit module
func (u *UserController) Permit(userID uint, resource string, accessLevel string) error {
	return u.userService.Permit(userID, resource, accessLevel)
}

func (u *UserController) GetPermit(userID string, resource string) (*entity.Permission, error) {
	return u.userService.GetPermit(userID, resource)
}

func (u *UserController) RevokePermit(userID string, resource string) error {
	return u.userService.RevokePermit(userID, resource)
}

func (u *UserController) ListPermits(userID string) ([]*entity.Permission, error) {
	return u.userService.ListPermits(userID)
}
