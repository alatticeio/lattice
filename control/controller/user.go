package controller

import (
	"linkany/control/entity"
	"linkany/control/mapper"
)

type UserController struct {
	userMapper mapper.UserInterface
}

func NewUserController(userMapper mapper.UserInterface) *UserController {
	return &UserController{userMapper: userMapper}
}

func (u *UserController) Login(user *entity.User) (*entity.Token, error) {
	return u.userMapper.Login(user)
}
