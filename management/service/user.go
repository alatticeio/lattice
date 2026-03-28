package service

import (
	"context"
	"errors"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/vo"
	"wireflow/pkg/utils"

	"gorm.io/gorm"
)

type UserService interface {
	InitAdmin(ctx context.Context, admins []config.AdminConfig) error
	Register(ctx context.Context, userDto dto.UserDto) error
	Login(ctx context.Context, email, password string) (*models.User, error)
	GetMe(ctx context.Context, id string) (*models.User, error)
	List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error)

	OnboardExternalUser(ctx context.Context, subject string, email string) (*models.User, error)
	AddUser(ctx context.Context, dtos *dto.UserDto) error
	DeleteUser(ctx context.Context, username string) error
}

type userService struct {
	log   *log.Logger
	store store.Store
}

func (u *userService) DeleteUser(ctx context.Context, id string) error {
	return u.store.Users().Delete(ctx, id)
}

func (u *userService) AddUser(ctx context.Context, dto *dto.UserDto) error {
	return u.store.Tx(ctx, func(s store.Store) error {
		newUser := &models.User{
			Username: dto.Username,
			Password: dto.Password,
			Role:     dto.Role,
		}
		if err := s.Users().Create(ctx, newUser); err != nil {
			return err
		}
		ws, err := s.Workspaces().GetByNamespace(ctx, dto.Namespace)
		if err != nil {
			return err
		}
		return s.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
			Role:        dto.Role,
			Status:      "active",
			WorkspaceID: ws.ID,
			UserID:      newUser.ID,
		})
	})
}

func (u *userService) OnboardExternalUser(ctx context.Context, subject string, email string) (*models.User, error) {
	existing, err := u.store.Users().GetByExternalID(ctx, subject)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	user := &models.User{ExternalID: subject, Email: email}
	return user, u.store.Users().Create(ctx, user)
}

func (u *userService) List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error) {
	return u.store.Users().List(ctx, req)
}

func (u *userService) InitAdmin(ctx context.Context, admins []config.AdminConfig) error {
	for _, admin := range admins {
		existing, err := u.store.Users().GetByUsername(ctx, admin.Username)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existing == nil {
			newUser := models.User{
				Username: admin.Username,
				Password: admin.Password,
				Role:     dto.RoleAdmin,
			}
			if err = u.store.Users().Create(ctx, &newUser); err != nil {
				u.log.Error("初始化管理员失败", err)
			} else {
				u.log.Info("✅ 初始管理员账号创建成功", "username", newUser.Username)
			}
		}
	}
	return nil
}

func (u *userService) GetMe(ctx context.Context, id string) (*models.User, error) {
	return u.store.Users().GetByID(ctx, id)
}

func (u *userService) Register(ctx context.Context, userDto dto.UserDto) error {
	password, err := utils.EncryptPassword(userDto.Password)
	if err != nil {
		return err
	}
	return u.store.Users().Create(ctx, &models.User{
		Username: userDto.Username,
		Password: password,
	})
}

func (u *userService) Login(ctx context.Context, username, password string) (*models.User, error) {
	user, err := u.store.Users().Login(ctx, username, password)
	if err != nil {
		return nil, errors.New("用户不存在或密码错误")
	}
	return user, nil
}

func NewUserService(st store.Store) UserService {
	return &userService{
		log:   log.GetLogger("user-service"),
		store: st,
	}
}
