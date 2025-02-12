package mapper

import (
	"context"
	"errors"
	"github.com/pion/turn/v4"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/redis"
)

var (
	_ UserInterface = (*UserMapper)(nil)
)

type UserMapper struct {
	*DatabaseService
	tokener *utils.Tokener
	rdb     *redis.Client
}

func NewUserMapper(db *DatabaseService, rdb *redis.Client) *UserMapper {
	return &UserMapper{DatabaseService: db, tokener: utils.NewTokener(), rdb: rdb}
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

	// Save turn key to redis
	key := turn.GenerateAuthKey(user.Username, "linkany.io", dto.Password)
	if err = u.rdb.Set(context.Background(), user.Username, string(key)); err != nil {
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
func (u *UserMapper) Get(token string) (*entity.User, error) {
	userToken, err := u.tokener.Parse(token)
	if err != nil {
		return nil, err
	}

	var user entity.User
	if err := u.Where("username = ?", userToken.Username).Find(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
