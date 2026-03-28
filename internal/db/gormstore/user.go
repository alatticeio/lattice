package gormstore

import (
	"context"

	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/repository"
	"wireflow/management/vo"

	"gorm.io/gorm"
)

type userRepo struct {
	*repository.BaseRepository[models.User]
}

func newUserRepo(db *gorm.DB) *userRepo {
	return &userRepo{BaseRepository: repository.NewBaseRepository[models.User](db)}
}

func (r *userRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	return r.BaseRepository.GetByID(ctx, id)
}

func (r *userRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return r.First(ctx, repository.WithUsername(username))
}

func (r *userRepo) GetByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	return r.First(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("external_id = ?", externalID)
	})
}

func (r *userRepo) Login(ctx context.Context, username, password string) (*models.User, error) {
	return r.First(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("username = ? AND password = ?", username, password)
	})
}

func (r *userRepo) Delete(ctx context.Context, id string) error {
	return r.BaseRepository.Delete(ctx, repository.WithID(id))
}

func (r *userRepo) Count(ctx context.Context) (int64, error) {
	return r.BaseRepository.Count(ctx)
}

func (r *userRepo) List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error) {
	var users []models.User
	var total int64

	query := r.DB().WithContext(ctx).Model(&models.User{})
	if req.Keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	err := query.Preload("Workspaces").
		Limit(req.PageSize).
		Offset((req.Page - 1) * req.PageSize).
		Order("created_at DESC").
		Find(&users).Error
	if err != nil {
		return nil, err
	}

	var userVos []vo.UserVo
	for _, u := range users {
		uvo := vo.UserVo{
			ID:       u.ID,
			Username: u.Username,
			Email:    u.Email,
			Avatar:   u.Avatar,
			Role:     string(u.Role),
		}
		for _, ws := range u.Workspaces {
			uvo.Workspaces = append(uvo.Workspaces, vo.WorkspaceVo{
				ID:          ws.ID,
				Slug:        ws.Slug,
				DisplayName: ws.DisplayName,
			})
		}
		userVos = append(userVos, uvo)
	}

	return &dto.PageResult[vo.UserVo]{
		List:     userVos,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}
