package service

import (
	"linkany/control/entity"
)

type UserInterface interface {
	Login(u *entity.User) (entity.Token, error)
}

type UserService struct {
	userMapper UserMapperInterface
}

type UserServiceConfig struct {
}

func NewUserService(cfg *UserServiceConfig) *UserService {
	return &UserService{}
}

func (u *UserService) Login(user *entity.User) (entity.Token, error) {
	return u.userMapper.Login(user)
}
