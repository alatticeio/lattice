package repository

import (
	"context"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/vo"

	"gorm.io/gorm"
)

type UserRepository struct {
	log *log.Logger
	*BaseRepository[models.User]
}

func (r *UserRepository) Login(ctx context.Context, username, password string) (*models.User, error) {

	var user models.User
	if err := r.db.WithContext(ctx).First(&user, "username = ? AND password = ?", username, password).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) OnboardExternalUser(ctx context.Context, subject string, email string) (*models.User, error) {

	user := &models.User{
		Email: email,
	}

	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error) {
	var users []models.User
	var total int64
	var userVos []vo.UserVo

	// 1. Initialize db handle
	query := r.db.WithContext(ctx).Model(&models.User{})

	// 2. If there are search conditions (e.g. search by username)
	if req.Keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}

	// 3. Count total (note: Count must be called before Limit/Offset)
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 4. Execute pagination
	err := query.Debug().
		Limit(req.PageSize).
		Offset((req.Page - 1) * req.PageSize).
		Order("created_at DESC").
		Find(&users).Error

	if err != nil {
		return nil, err
	}

	// 5. Convert to VO (Value Object)
	for _, user := range users {
		userVo := vo.UserVo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Avatar:   user.Avatar,
			Role:     string(user.SystemRole),
		}

		userVos = append(userVos, userVo)
	}

	// 6. Return standard paginated result
	return &dto.PageResult[vo.UserVo]{
		List:     userVos,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		log:            log.GetLogger("user-repository"),
		BaseRepository: NewBaseRepository[models.User](db),
	}
}
