package controller

import (
	"context"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/service"
	"wireflow/management/vo"
)

type UserController interface {
	InitAdmin(ctx context.Context, admins []config.AdminConfig) error
	Register(ctx context.Context, userDto dto.UserDto) error
	Login(ctx context.Context, email, password string) (*models.User, error)
	GetMe(ctx context.Context, id string) (*models.User, error)

	AddUser(ctx context.Context, userDto *dto.UserDto) error
	DeleteUser(ctx context.Context, username string) error

	ListUser(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error)

	AssignPermission(ctx context.Context, id string, userDto dto.UserDto) error
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

func (u *userController) GetMe(ctx context.Context, id string) (*models.User, error) {
	return u.userService.GetMe(ctx, id)
}

func (u *userController) Login(ctx context.Context, email, password string) (*models.User, error) {
	return u.userService.Login(ctx, email, password)
}

func NewUserController(st store.Store) UserController {
	return &userController{
		log:         log.GetLogger("user-controller"),
		userService: service.NewUserService(st),
	}
}

func (u *userController) Register(ctx context.Context, userDto dto.UserDto) error {
	return u.userService.Register(ctx, userDto)
}
