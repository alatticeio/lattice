package repository

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/model"

	"gorm.io/gorm"
)

type UserRepository interface {
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
}

type userRepository struct {
	db *gorm.DB
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
			Email:    user.Email,
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
	return &userRepository{db: db}
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
