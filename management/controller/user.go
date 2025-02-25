package controller

import (
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/pkg/log"
)

type UserController struct {
	logger     *log.Logger
	userMapper service.UserService
}

func NewUserController(userMapper service.UserService) *UserController {
	return &UserController{userMapper: userMapper, logger: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "user-controller"))}
}

func (u *UserController) Login(dto *dto.UserDto) (*entity.Token, error) {
	return u.userMapper.Login(dto)
}

func (u *UserController) Register(e *dto.UserDto) (*entity.User, error) {
	return u.userMapper.Register(e)
}

func (u *UserController) Get(token string) (*entity.User, error) {
	return u.userMapper.Get(token)
}
