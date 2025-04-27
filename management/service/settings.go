package service

import (
	"context"
	"gorm.io/gorm"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/repository"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type UserSettingsService interface {
	// NewAppKey create app
	NewAppKey(ctx context.Context) error

	// RemoveAppKey delete app
	RemoveAppKey(ctx context.Context, keyId uint64) error

	UpdateAppKey(ctx context.Context, dto *dto.AppKeyDto) error

	NewUserSettings(ctx context.Context, dto *dto.UserSettingsDto) error

	ListAppKeys(ctx context.Context, params *dto.AppKeyParams) (*vo.PageVo, error)
}

var (
	_ UserSettingsService = (*userSettingsServiceImpl)(nil)
)

type userSettingsServiceImpl struct {
	logger      *log.Logger
	db          *gorm.DB
	settingRepo repository.SettingRepository
}

func NewUserSettingsService(db *gorm.DB) UserSettingsService {
	logger := log.NewLogger(log.Loglevel, "user-settings-service")
	return &userSettingsServiceImpl{logger: logger, db: db, settingRepo: repository.NewSettingRepository(db)}
}

func (a *userSettingsServiceImpl) NewAppKey(ctx context.Context) error {
	return a.settingRepo.CreateAppKey(ctx, &entity.AppKey{AppKey: utils.GenerateUUID(),
		UserId: utils.GetUserIdFromCtx(ctx), Status: entity.Active})
}

func (a *userSettingsServiceImpl) RemoveAppKey(ctx context.Context, keyId uint64) error {
	return a.settingRepo.DeleteAppKey(ctx, keyId)
}

func (a *userSettingsServiceImpl) NewUserSettings(ctx context.Context, dto *dto.UserSettingsDto) error {
	return a.settingRepo.CreateUserSetting(ctx, &entity.UserSettings{
		AppKey:     dto.AppKey,
		PlanType:   dto.PlanType,
		NodeLimit:  dto.NodeLimit,
		NodeFree:   dto.NodeFree,
		GroupLimit: dto.GroupLimit,
	})
}

func (a *userSettingsServiceImpl) UpdateAppKey(ctx context.Context, dto *dto.AppKeyDto) error {
	return a.settingRepo.UpdateAppKey(ctx, &entity.AppKey{
		Model: entity.Model{
			ID: dto.ID,
		},
		Status: dto.Status,
	})
}

func (a *userSettingsServiceImpl) ListAppKeys(ctx context.Context, params *dto.AppKeyParams) (*vo.PageVo, error) {
	var (
		err             error
		userSettingsKey []*entity.AppKey
		count           int64
		result          = new(vo.PageVo)
	)

	if userSettingsKey, count, err = a.settingRepo.ListAppKeys(ctx, params); err != nil {
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
