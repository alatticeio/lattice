package mapper

import (
	"errors"
	"linkany/control/dto"
	"linkany/control/entity"
	"linkany/control/utils"
)

var (
	_ UserInterface = (*UserMapper)(nil)
)

type UserMapper struct {
	*DatabaseService
	tokener *utils.Tokener
}

func NewUserMapper(db *DatabaseService) *UserMapper {
	return &UserMapper{DatabaseService: db, tokener: utils.NewTokener()}
}

// Login checks if the user exists and returns a token
func (u *UserMapper) Login(dto *dto.UserDto) (*entity.Token, error) {
	var user entity.User
	if err := u.Where("username = ?", dto.Username).First(&user).Error; err != nil {
		return nil, errors.New("user not found")
	}

	if err := utils.ComparePassword(user.Password, dto.Password); err != nil {
		return nil, errors.New("invalid password")
	}

	token, err := u.tokener.Generate(user.Username, user.Password)
	if err != nil {
		return nil, err
	}
	return &entity.Token{Token: token}, nil
}

// Register creates a new user
func (u *UserMapper) Register(dto *dto.UserDto) (*entity.User, error) {
	hashedPassword, err := utils.EncryptPassword(dto.Password)
	if err != nil {
		return nil, err
	}
	e := &entity.User{
		Username: dto.Username,
		Password: hashedPassword,
	}
	err = u.Create(e).Error
	if err != nil {
		return nil, err
	}
	return e, nil
}

// Get returns a user by username
func (u *UserMapper) Get(username string) (*entity.User, error) {
	var user entity.User
	if err := u.Where("username = ?", username).Find(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
