package mapper

import (
	"linkany/control/dto"
	"linkany/control/entity"
)

type UserInterface interface {
	Login(u *dto.UserDto) (*entity.Token, error)
	Register(e *dto.UserDto) (*entity.User, error)
}
