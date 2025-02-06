package controller

import (
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/mapper"
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

func (u *UserController) Get(token string) (*entity.User, error) {
	return u.userMapper.Get(token)
}
