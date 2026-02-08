package controller

import (
	"context"
	"wireflow/internal/log"
	"wireflow/management/dto"
	"wireflow/management/model"
	"wireflow/management/service"
)

type UserController interface {
	Register(ctx context.Context, userDto dto.UserDto) error
	Login(ctx context.Context, email, password string) (string, error)
	GetMe(ctx context.Context, id string) (*model.User, error)
}

var (
	_ UserController = (*userController)(nil)
)

type userController struct {
	log         *log.Logger
	userService service.UserService
}

func (u userController) GetMe(ctx context.Context, id string) (*model.User, error) {
	return u.userService.GetMe(ctx, id)
}

func (u userController) Login(ctx context.Context, email, password string) (string, error) {
	return u.userService.Login(ctx, email, password)
}

func NewUserController() UserController {
	return &userController{
		log:         log.GetLogger("user-controller"),
		userService: service.NewUserService(),
	}
}

func (u userController) Register(ctx context.Context, userDto dto.UserDto) error {
	return u.userService.Register(ctx, userDto)
}
