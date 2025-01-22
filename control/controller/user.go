package controller

import (
	"linkany/control/dto"
	"linkany/control/entity"
	"linkany/control/mapper"
)

type UserController struct {
	userMapper mapper.UserInterface
}

func NewUserController(userMapper mapper.UserInterface) *UserController {
	return &UserController{userMapper: userMapper}
}

func (u *UserController) Login(dto *dto.UserDto) (*entity.Token, error) {
	return u.userMapper.Login(dto)
}

func (u *UserController) Register(e *dto.UserDto) (*entity.User, error) {
	return u.userMapper.Register(e)
}

func (u *UserController) Get(username string) (*entity.User, error) {
	return u.userMapper.Get(username)
}
