package service

import (
	"context"
	"errors"
	"wireflow/internal/log"
	"wireflow/management/database"
	"wireflow/management/dto"
	"wireflow/management/repository"
	"wireflow/pkg/utils"

	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Register(ctx context.Context, userDto dto.UserDto) error
	Login(ctx context.Context, email, password string) (string, error)
}

type userService struct {
	log            *log.Logger
	userRepository repository.UserRepository
}

func (u userService) Register(ctx context.Context, userDto dto.UserDto) error {
	var err error
	userDto.Password, err = utils.EncryptPassword(userDto.Password)
	if err != nil {
		return err
	}
	return u.userRepository.Register(ctx, &userDto)
}

func (s *userService) Login(ctx context.Context, email, password string) (string, error) {
	// 1. 调用 Repository 获取用户
	user, err := s.userRepository.GetByEmail(ctx, email)
	if err != nil {
		return "", errors.New("用户不存在或密码错误")
	}

	// 核心校验步骤：
	// 第一个参数是数据库里的密文，第二个参数是用户输入的明文
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", errors.New("用户不存在或密码错误")
	}

	// 3. 生成 Token
	token, err := utils.GenerateToken(uint(user.ID))
	if err != nil {
		return "", errors.New("生成 Token 失败")
	}

	return token, nil
}

func NewUserService() UserService {
	return &userService{
		log:            log.GetLogger("user-service"),
		userRepository: repository.NewUserRepository(database.DB),
	}
}
