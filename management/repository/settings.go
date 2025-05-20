package repository

import (
	"context"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"
)

type SettingRepository interface {
	WithTx(tx *gorm.DB) SettingRepository
	CreateAppKey(ctx context.Context, appKey *entity.AppKey) error
	DeleteAppKey(ctx context.Context, id uint64) error
	UpdateAppKey(ctx context.Context, appKey *entity.AppKey) error

	ListAppKeys(ctx context.Context, params *dto.AppKeyParams) ([]*entity.AppKey, int64, error)

	CreateUserSetting(ctx context.Context, userSetting *entity.UserSettings) error
}

var (
	_ SettingRepository = (*settingRepository)(nil)
)

type settingRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewSettingRepository(db *gorm.DB) SettingRepository {
	return &settingRepository{db: db, logger: log.NewLogger(log.Loglevel, "setting-repository")}
}

func (s settingRepository) WithTx(tx *gorm.DB) SettingRepository {
	return NewSettingRepository(tx)
}

func (s settingRepository) CreateAppKey(ctx context.Context, appKey *entity.AppKey) error {
	return s.db.WithContext(ctx).Create(appKey).Error
}

func (s settingRepository) DeleteAppKey(ctx context.Context, id uint64) error {
	return s.db.WithContext(ctx).Delete(&entity.AppKey{}, id).Error
}

func (s settingRepository) UpdateAppKey(ctx context.Context, appKey *entity.AppKey) error {
	return s.db.WithContext(ctx).Model(&entity.AppKey{}).Where("id = ?", appKey.ID).Update("status", appKey.Status).Error
}

func (s settingRepository) ListAppKeys(ctx context.Context, params *dto.AppKeyParams) ([]*entity.AppKey, int64, error) {
	var (
		err   error
		count int64
	)
	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(s.db.WithContext(ctx).Model(&entity.AppKey{}))

	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query.Offset(int(pageOffset.Offset)).Limit(int(pageOffset.Limit))
	}

	var appKeys []*entity.AppKey
	if err := query.Find(&appKeys).Error; err != nil {
		return nil, 0, err
	}

	return appKeys, count, nil
}

func (s settingRepository) CreateUserSetting(ctx context.Context, userSetting *entity.UserSettings) error {
	return s.db.WithContext(ctx).Create(userSetting).Error
}
