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

	// 1. 初始化 db 句柄
	query := r.db.WithContext(ctx).Model(&models.User{})

	// 2. 如果有搜索条件（例如按用户名搜索）
	if req.Keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}

	// 3. 统计总数（注意：Count 必须在 Limit/Offset 之前执行）
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 4. 执行分页
	err := query.Debug().
		Limit(req.PageSize).
		Offset((req.Page - 1) * req.PageSize).
		Order("created_at DESC").
		Find(&users).Error

	if err != nil {
		return nil, err
	}

	// 5. 转换为 VO (Value Object)
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

	// 6. 返回标准分页结果
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
