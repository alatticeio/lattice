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
}

type userService struct {
	log                 *log.Logger
	db                  *gorm.DB
	userRepository      repository.UserRepository
	workspaceRepo       repository.WorkspaceRepository
	workspaceMemberRepo repository.WorkspaceMemberRepository
}

func (u *userService) AddUser(ctx context.Context, dto *dto.UserDto) error {
	// 先创建user
	return u.db.Transaction(func(tx *gorm.DB) error {
		userId := ctx.Value("user_id").(string)
		userRepo := u.userRepository.WithTx(tx)
		err := userRepo.AddUser(ctx, dto)
		if err != nil {
			return err
		}

		workspaceRepo := u.workspaceRepo.WithTx(tx)

		ws, err := workspaceRepo.FindByNs(ctx, dto.Namespace)
		if err != nil {
			return err
		}

		//创建workspace member
		workspaceMember := model.WorkspaceMember{
			Role:        dto.Role,
			Status:      "active",
			WorkspaceID: ws.ID,
			UserID:      userId,
		}

		_, err = u.workspaceMemberRepo.Create(ctx, &workspaceMember)
		if err != nil {
			return err
		}

		return nil
	})
}

func (u *userService) OnboardExternalUser(ctx context.Context, subject string, email string) (*model.User, error) {
	return u.userRepository.OnboardExternalUser(ctx, subject, email)
}

func (u *userService) List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error) {
	return u.userRepository.List(ctx, req)
}

func (u *userService) InitAdmin(ctx context.Context, admins []config.AdminConfig) error {
	return u.userRepository.InitAdmin(ctx, admins)
}

func (u *userService) GetMe(ctx context.Context, id string) (*model.User, error) {
	return u.userRepository.GetMe(ctx, id)
}

func (u *userService) Register(ctx context.Context, userDto dto.UserDto) error {
	var err error
	userDto.Password, err = utils.EncryptPassword(userDto.Password)
	if err != nil {
		return err
	}
	return u.userRepository.Register(ctx, &userDto)
}

func (s *userService) Login(ctx context.Context, username, password string) (*model.User, error) {
	// 1. 调用 Repository 获取用户
	user, err := s.userRepository.Login(ctx, username, password)
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
		userRepository:      repository.NewUserRepository(database.DB),
		workspaceMemberRepo: repository.NewWorkspaceMemberRepository(),
		workspaceRepo:       repository.NewWorkspaceRepository(),
	}
}
