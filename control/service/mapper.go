package service

import "linkany/control/entity"

// UserMapperInterface is an interface for user mapper
type UserMapperInterface interface {
	Login(u *entity.User) (entity.Token, error)
}

type UserMapper struct {
	db *DatabaseService
}

func NewUserMapper(db *DatabaseService) *UserMapper {
	return &UserMapper{db: db}
}

func (u *UserMapper) Login(user *entity.User) (entity.Token, error) {
	var token entity.Token
	err := u.db.GetDB().Where("user_name = ? AND password = ?", user.UserName, user.Password).First(&token).Error
	if err != nil {
		return token, err
	}

	return token, nil
}
