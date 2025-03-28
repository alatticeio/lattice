package service

import (
	"context"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type UserSettingsService interface {
	// NewAppKey create app
	NewAppKey(ctx context.Context) error

	// RemoveAppKey delete app
	RemoveAppKey(ctx context.Context, keyId uint) error

	UpdateAppKey(ctx context.Context, dto *dto.AppKeyDto) error

	NewUserSettings(ctx context.Context, dto *dto.UserSettingsDto) error

	ListAppKeys(ctx context.Context, params *dto.AppKeyParams) (*vo.PageVo, error)
}

type userSettingsServiceImpl struct {
	logger *log.Logger
	*DatabaseService
}

func NewUserSettingsService(db *DatabaseService) UserSettingsService {
	logger := log.NewLogger(log.Loglevel, "user-settings-service")
	return &userSettingsServiceImpl{logger: logger, DatabaseService: db}
}

func (a userSettingsServiceImpl) NewAppKey(ctx context.Context) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		if err = tx.Model(&entity.AppKey{}).Create(&entity.AppKey{AppKey: utils.GenerateUUID(),
			UserId: utils.GetUserIdFromCtx(ctx), Status: entity.Active}).Error; err != nil {
			return err
		}

		return nil
	})
}

func (a userSettingsServiceImpl) RemoveAppKey(ctx context.Context, keyId uint) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		if err = tx.Model(&entity.AppKey{}).Where("id = ?", keyId).Delete(&entity.AppKey{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (a userSettingsServiceImpl) NewUserSettings(ctx context.Context, dto *dto.UserSettingsDto) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		if err = tx.Model(&entity.UserSettings{}).Create(&entity.UserSettings{
			AppKey:     dto.AppKey,
			PlanType:   dto.PlanType,
			NodeLimit:  dto.NodeLimit,
			NodeFree:   dto.NodeFree,
			GroupLimit: dto.GroupLimit,
		}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (a userSettingsServiceImpl) UpdateAppKey(ctx context.Context, dto *dto.AppKeyDto) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		if err = tx.Model(&entity.AppKey{}).Where("id = ?", dto).Update("status", dto.Status).Error; err != nil {
			return err
		}
		return nil
	})
}

func (a userSettingsServiceImpl) ListAppKeys(ctx context.Context, params *dto.AppKeyParams) (*vo.PageVo, error) {
	var (
		err             error
		userSettingsKey []entity.AppKey
		count           int64
	)
	sql, wrappers := utils.Generate(params)
	result := new(vo.PageVo)
	db := a.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	if err = db.Model(&entity.AppKey{}).Count(&count).Error; err != nil {
		return nil, err
	}

	if err := db.Model(&entity.AppKey{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&userSettingsKey).Error; err != nil {
		return nil, err
	}

	var userSettingsKeyVo []*vo.AppKeyVo
	for _, key := range userSettingsKey {
		userSettingsKeyVo = append(userSettingsKeyVo, &vo.AppKeyVo{
			AppKey: key.AppKey,
			Status: key.Status.String(),
			ModelVo: vo.ModelVo{
				ID:        key.ID,
				CreatedAt: key.CreatedAt,
				UpdatedAt: key.UpdatedAt,
			},
		})
	}

	result.Data = userSettingsKeyVo
	result.Page = params.Page
	result.Size = params.Size
	result.Total = count

	return result, nil
}
