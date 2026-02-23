package repository

import (
	"context"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/management/dto"
	"wireflow/management/model"
	"wireflow/management/vo"

	"gorm.io/gorm"
)

type UserRepository interface {
	InitAdmin(ctx context.Context, admins []config.AdminConfig) error

	AddUser(ctx context.Context, userDto *dto.UserDto) error

	GetMe(ctx context.Context, id string) (*model.User, error)

	// 基础查询
	GetByID(ctx context.Context, id uint) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)

	Register(ctx context.Context, user *dto.UserDto) error

	// 核心注册逻辑：创建用户并初始化环境
	// 使用事务确保用户和默认网络同时成功
	CreateWithDefaultNetwork(ctx context.Context, user *model.User, networkName string) error

	// 其他管理操作
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id uint) error

	// ListUser
	List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error)

	OnboardExternalUser(ctx context.Context, subject string, email string) (*model.User, error)
	Login(ctx context.Context, username, password string) (*model.User, error)
	WithTx(tx *gorm.DB) UserRepository
}

type userRepository struct {
	log *log.Logger
	db  *gorm.DB
}

func (r *userRepository) WithTx(tx *gorm.DB) UserRepository {
	return &userRepository{
		db: tx,
	}
}

func (r *userRepository) AddUser(ctx context.Context, userDto *dto.UserDto) error {
	user := model.User{
		Username: userDto.Username,
		Password: userDto.Password,
		Email:    userDto.Email,
		Role:     userDto.Role,
	}

	if err := r.db.WithContext(ctx).Create(&user).Error; err != nil {
		return err
	}

	return nil
}

func (r *userRepository) Login(ctx context.Context, username, password string) (*model.User, error) {

	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "username = ? AND password = ?", username, password).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *userRepository) OnboardExternalUser(ctx context.Context, subject string, email string) (*model.User, error) {

	user := &model.User{
		Email:      email,
		ExternalID: subject,
	}

	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepository) List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error) {
	var users []model.User
	var total int64
	var userVos []vo.UserVo

	// 1. 初始化 db 句柄
	query := r.db.WithContext(ctx).Model(&model.User{})

	// 2. 如果有搜索条件（例如按用户名搜索）
	if req.Keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}

	// 3. 统计总数（注意：Count 必须在 Limit/Offset 之前执行）
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 4. 执行分页与关联预加载
	// 假设你想在用户列表里展示他们所属的 Workspaces
	err := query.
		Preload("Workspaces").
		Limit(req.PageSize).
		Offset((req.Page - 1) * req.PageSize).
		Order("created_at DESC").
		Find(&users).Error

	if err != nil {
		return nil, err
	}

	// 5. 转换为 VO (Value Object)
	// 实际项目中建议使用 copier 库或手动映射
	for _, user := range users {
		userVos = append(userVos, vo.UserVo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Avatar:   user.Avatar,
			// 可以在这里提取所属 Workspace 的名称列表
		})
	}

	// 6. 返回标准分页结果
	return &dto.PageResult[vo.UserVo]{
		List:     userVos,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (r *userRepository) InitAdmin(ctx context.Context, admins []config.AdminConfig) error {
	for _, admin := range admins {
		var count int64
		// 1. 检查是否存在名为 admin 的用户
		err := r.db.Model(&model.User{}).Where("username = ?", admin.Username).Count(&count).Error
		if err != nil {
			return err
		}

		if count == 0 {
			// 2. 不存在则创建
			admin := model.User{
				Username: admin.Username,
				Password: admin.Password, // 记得加密！
				Role:     dto.RoleAdmin,
			}
			if err := r.db.Create(&admin).Error; err != nil {
				r.log.Error("初始化管理员失败", err)
			} else {
				r.log.Info("✅ 初始管理员账号创建成功", "username", admin.Username)
			}
		}
	}

	return nil
}

func (r *userRepository) GetMe(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *userRepository) GetByID(ctx context.Context, id uint) (*model.User, error) {
	//TODO implement me
	panic("implement me")
}

func (r *userRepository) Register(ctx context.Context, user *dto.UserDto) error {
	// 开启事务
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		// 1. 创建用户
		if err := tx.Create(&model.User{
			Email:    user.Username,
			Password: user.Password,
		}).Error; err != nil {
			return err // 事务回滚
		}

		return nil
	})
}

func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	//TODO implement me
	panic("implement me")
}

func (r *userRepository) Delete(ctx context.Context, id uint) error {
	//TODO implement me
	panic("implement me")
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		log: log.GetLogger("user-repository"),
		db:  db}
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	return &user, err
}

func (r *userRepository) CreateWithDefaultNetwork(ctx context.Context, user *model.User, networkName string) error {
	// 开启事务
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		return nil
	})
}
