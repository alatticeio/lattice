package service

import (
	"context"
	"errors"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/management/database"
	"wireflow/management/dto"
	"wireflow/management/model"
	"wireflow/management/repository"
	"wireflow/management/vo"
	"wireflow/pkg/utils"

	"gorm.io/gorm"
)

type UserService interface {
	InitAdmin(ctx context.Context, admins []config.AdminConfig) error
	Register(ctx context.Context, userDto dto.UserDto) error
	Login(ctx context.Context, email, password string) (*model.User, error)
	GetMe(ctx context.Context, id string) (*model.User, error)
	List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error)

	OnboardExternalUser(ctx context.Context, subject string, email string) (*model.User, error)
	AddUser(ctx context.Context, dtos *dto.UserDto) error
	DeleteUser(ctx context.Context, username string) error
}

type userService struct {
	log                 *log.Logger
	db                  *gorm.DB
	userRepo            *repository.UserRepository
	workspaceRepo       *repository.WorkspaceRepository
	workspaceMemberRepo *repository.WorkspaceMemberRepository
}

func (u *userService) DeleteUser(ctx context.Context, id string) error {
	return u.userRepo.Delete(ctx, repository.WithID(id))
}

func (u *userService) AddUser(ctx context.Context, dto *dto.UserDto) error {
	// 先创建user
	return u.db.Transaction(func(tx *gorm.DB) error {
		userRepo := repository.NewUserRepository(tx)
		newUser := &model.User{
			Username: dto.Username,
			Password: dto.Password,
			Role:     dto.Role,
		}
		err := userRepo.Create(ctx, newUser)
		if err != nil {
			return err
		}

		workspaceRepo := repository.NewWorkspaceRepository(tx)

		ws, err := workspaceRepo.First(ctx, repository.WithNamespace(dto.Namespace))
		if err != nil {
			return err
		}

		//创建workspace member
		workspaceMember := model.WorkspaceMember{
			Role:        dto.Role,
			Status:      "active",
			WorkspaceID: ws.ID,
			UserID:      newUser.ID,
		}

		workspaceMemberRepo := repository.NewWorkspaceMemberRepository(tx)

		err = workspaceMemberRepo.Create(ctx, &workspaceMember)
		if err != nil {
			return err
		}

		return nil
	})
}

func (u *userService) OnboardExternalUser(ctx context.Context, subject string, email string) (*model.User, error) {
	return u.userRepo.OnboardExternalUser(ctx, subject, email)
}

func (u *userService) List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error) {
	return u.userRepo.List(ctx, req)
}

func (u *userService) InitAdmin(ctx context.Context, admins []config.AdminConfig) error {
	for _, admin := range admins {
		// 1. 检查是否存在名为 admin 的用户
		count, err := u.userRepo.Count(ctx, repository.WithUsername(admin.Username))
		if err != nil {
			return err
		}

		if count == 0 {
			// 2. 不存在则创建
			newUser := model.User{
				Username: admin.Username,
				Password: admin.Password, // 记得加密！
				Role:     dto.RoleAdmin,
			}
			if err = u.userRepo.Create(ctx, &newUser); err != nil {
				u.log.Error("初始化管理员失败", err)
			} else {
				u.log.Info("✅ 初始管理员账号创建成功", "username", newUser.Username)
			}
		}
	}

	return nil
}

func (u *userService) GetMe(ctx context.Context, id string) (*model.User, error) {
	return u.userRepo.First(ctx, repository.WithID(id))
}

func (u *userService) Register(ctx context.Context, userDto dto.UserDto) error {
	return u.userRepo.WithTransaction(func(txRepo *repository.BaseRepository[model.User]) error {
		password, err := utils.EncryptPassword(userDto.Password)
		if err != nil {
			return err
		}

		return txRepo.Create(ctx, &model.User{
			Username: userDto.Username,
			Password: password,
		})
	})
}

func (s *userService) Login(ctx context.Context, username, password string) (*model.User, error) {
	// 1. 调用 Repository 获取用户
	user, err := s.userRepo.Login(ctx, username, password)
	if err != nil {
		return nil, errors.New("用户不存在或密码错误")
	}

	//// 核心校验步骤：
	//// 第一个参数是数据库里的密文，第二个参数是用户输入的明文
	//err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	//if err != nil {
	//	return nil, errors.New("用户不存在或密码错误")
	//}

	return user, nil
}

func (s *userService) Get() {

}

func NewUserService() UserService {
	return &userService{
		log:                 log.GetLogger("user-service"),
		db:                  database.DB,
		userRepo:            repository.NewUserRepository(database.DB),
		workspaceRepo:       repository.NewWorkspaceRepository(database.DB),
		workspaceMemberRepo: repository.NewWorkspaceMemberRepository(database.DB),
	}
}
