package controller

import (
	"context"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/management/dto"
	"wireflow/management/model"
	"wireflow/management/service"
	"wireflow/management/vo"
)

type UserController interface {
	InitAdmin(ctx context.Context, admins []config.AdminConfig) error
	Register(ctx context.Context, userDto dto.UserDto) error
	Login(ctx context.Context, email, password string) (*model.User, error)
	GetMe(ctx context.Context, id string) (*model.User, error)

	// Add user from admin
	AddUser(ctx context.Context, userDto *dto.UserDto) error
	DeleteUser(ctx context.Context, username string) error

	// management page, if current user is admin will list all users created by self,
	//if current is ns amdin, will list all users in the ns. other will list none.
	ListUser(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error)

	// admin will grant permit to user
	AssignPermission(ctx context.Context, id string, userDto dto.UserDto) error

	// admin will revoke permit from user
	RevokePermission(ctx context.Context, id string, userDto dto.UserDto) error
}

var (
	_ UserController = (*userController)(nil)
)

type userController struct {
	log         *log.Logger
	userService service.UserService
}

func (u *userController) DeleteUser(ctx context.Context, username string) error {
	return u.userService.DeleteUser(ctx, username)
}

func (u *userController) AddUser(ctx context.Context, userDto *dto.UserDto) error {
	return u.userService.AddUser(ctx, userDto)
}

func (u *userController) ListUser(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error) {
	return u.userService.List(ctx, req)
}

func (u *userController) AssignPermission(ctx context.Context, id string, userDto dto.UserDto) error {
	//TODO implement me
	panic("implement me")
}

func (u *userController) RevokePermission(ctx context.Context, id string, userDto dto.UserDto) error {
	//TODO implement me
	panic("implement me")
}

func (u *userController) InitAdmin(ctx context.Context, admins []config.AdminConfig) error {
	return u.userService.InitAdmin(ctx, admins)
}

func (u *userController) GetMe(ctx context.Context, id string) (*model.User, error) {
	return u.userService.GetMe(ctx, id)
}

func (u *userController) Login(ctx context.Context, email, password string) (*model.User, error) {
	return u.userService.Login(ctx, email, password)
}

func NewUserController() UserController {
	return &userController{
		log:         log.GetLogger("user-controller"),
		userService: service.NewUserService(),
	}
}

func (u *userController) Register(ctx context.Context, userDto dto.UserDto) error {
	return u.userService.Register(ctx, userDto)
}
