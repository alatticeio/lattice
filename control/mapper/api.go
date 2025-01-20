package mapper

import (
	"linkany/control/entity"
)

type UserInterface interface {
	Login(u *entity.User) (*entity.Token, error)
}
