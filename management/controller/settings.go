package controller

import (
	"context"
	"gorm.io/gorm"
	"wireflow/management/dto"
	"wireflow/management/service"
	"wireflow/management/vo"
	"wireflow/pkg/log"
)

type SettingsController struct {
	logger          *log.Logger
	settingsService service.UserSettingsService
}

func NewSettingsController(db *gorm.DB) *SettingsController {
	return &SettingsController{settingsService: service.NewUserSettingsService(db), logger: log.NewLogger(log.Loglevel, "settings-controller")}
}

func (s *SettingsController) NewAppKey(ctx context.Context) error {
	return s.settingsService.NewAppKey(ctx)
}

func (s *SettingsController) RemoveAppKey(ctx context.Context, id uint64) error {
	return s.settingsService.RemoveAppKey(ctx, id)
}

func (s *SettingsController) NewUserSettings(ctx context.Context, dto *dto.UserSettingsDto) error {
	return s.settingsService.NewUserSettings(ctx, dto)
}

func (s *SettingsController) ListAppkeys(ctx context.Context, params *dto.AppKeyParams) (*vo.PageVo, error) {
	return s.settingsService.ListAppKeys(ctx, params)
}
