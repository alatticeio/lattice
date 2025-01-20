package mapper

import (
	"linkany/control/entity"
	"linkany/control/utils"
)

type UserMapper struct {
	*DatabaseService
	tokener *utils.Tokener
}

func NewUserMapper(db *DatabaseService) *UserMapper {
	return &UserMapper{DatabaseService: db, tokener: utils.NewTokener()}
}

func (u *UserMapper) Login(user *entity.User) (*entity.Token, error) {
	var rUser entity.User
	err := u.Where("username = ? AND password = ?", user.UserName, user.Password).First(&rUser).Error
	if err != nil {
		return nil, err
	}

	token, err := u.tokener.Generate(rUser.UserName, rUser.Password)
	if err != nil {
		return nil, err
	}
	return &entity.Token{Token: token}, nil
}
