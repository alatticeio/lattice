package service

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type UserSettingsService interface {
	// CreateApp create app
	NewUserSettingsKey(ctx context.Context) error

	// DeleteApp delete app
	DeleteUserSettingsKey(ctx context.Context, keyId uint) error

	NewUserSettings(ctx context.Context, dto *dto.UserSettingsDto) error

	UserSettingsKeyList(ctx context.Context, params *dto.UserKeyParams) (*vo.PageVo, error)
}

type userSettingsServiceImpl struct {
	logger *log.Logger
	*DatabaseService
}

func NewUserSettingsService(db *DatabaseService) UserSettingsService {
	logger := log.NewLogger(log.Loglevel, fmt.Sprintf("[%s ],", "user-settings-service"))
	return &userSettingsServiceImpl{logger: logger, DatabaseService: db}
}

func (a userSettingsServiceImpl) NewUserSettingsKey(ctx context.Context) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		if err = tx.Model(&entity.UserSettingsKey{}).Create(&entity.UserSettingsKey{UserKey: utils.GenerateUUID(),
			UserId: utils.GetUserIdFromCtx(ctx),
		}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (a userSettingsServiceImpl) DeleteUserSettingsKey(ctx context.Context, keyId uint) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		if err = tx.Model(&entity.UserSettingsKey{}).Where("id = ?", keyId).Delete(&entity.UserSettingsKey{}).Error; err != nil {
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

func (a userSettingsServiceImpl) UserSettingsKeyList(ctx context.Context, params *dto.UserKeyParams) (*vo.PageVo, error) {
	var (
		err             error
		userSettingsKey []entity.UserSettingsKey
		count           int64
	)
	sql, wrappers := utils.Generate(params)
	result := new(vo.PageVo)
	db := a.DB
	if sql != "" {
		db = db.Where(sql, wrappers)
	}

	if err = db.Model(&entity.UserSettingsKey{}).Count(&count).Error; err != nil {
		return nil, err
	}

	if err := db.Model(&entity.UserSettingsKey{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&userSettingsKey).Error; err != nil {
		return nil, err
	}

	var userSettingsKeyVo []*vo.UserSettingsKeyVo
	for _, key := range userSettingsKey {
		userSettingsKeyVo = append(userSettingsKeyVo, &vo.UserSettingsKeyVo{
			UserSettingsKey: key.UserKey,
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
