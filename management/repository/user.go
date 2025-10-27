package repository

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/management/utils"
	"wireflow/pkg/log"

	"gorm.io/gorm"
)

type UserRepository interface {
	WithTx(tx *gorm.DB) UserRepository
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	GetByUsernames(ctx context.Context, usernames []string) ([]*entity.User, error)
	Create(ctx context.Context, user *entity.User) error

	List(ctx context.Context, params *dto.UserParams) ([]*entity.User, int64, error)
	Query(ctx context.Context, params *dto.UserParams) ([]*entity.User, error)
}

var (
	_ UserRepository = (*userRepository)(nil)
)

type userRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func (r *userRepository) Query(ctx context.Context, params *dto.UserParams) ([]*entity.User, error) {
	var (
		err   error
		users []*entity.User
	)
	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.User{}))

	if err = query.Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func (r *userRepository) List(ctx context.Context, params *dto.UserParams) ([]*entity.User, int64, error) {
	var (
		err   error
		users []*entity.User
		count int64
	)

	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.User{}))

	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err = query.Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, count, nil
}

func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	var user entity.User
	if err := r.db.WithContext(ctx).Model(&entity.User{}).Where("username = ?", username).Find(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *userRepository) GetByUsernames(ctx context.Context, usernames []string) ([]*entity.User, error) {
	var user []*entity.User
	if err := r.db.WithContext(ctx).Model(&entity.User{}).Where("username in ?", usernames).Find(&user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "shared-repository"),
	}
}

func (r *userRepository) WithTx(tx *gorm.DB) UserRepository {
	return NewUserRepository(tx)
}
