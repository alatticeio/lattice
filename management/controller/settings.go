package controller

import (
	"context"
	"fmt"
	"linkany/management/dto"
	"linkany/management/service"
	"linkany/management/vo"
	"linkany/pkg/log"
)

type SettingsController struct {
	logger          *log.Logger
	settingsService service.UserSettingsService
}

func NewSettingsController(settingsService service.UserSettingsService) *SettingsController {
	return &SettingsController{settingsService: settingsService, logger: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "settings-controller"))}
}

func (s *SettingsController) NewUserSettingsKey(ctx context.Context) error {
	return s.settingsService.NewUserSettingsKey(ctx)
}

func (s *SettingsController) DeleteUserSettingsKey(ctx context.Context, id uint) error {
	return s.settingsService.DeleteUserSettingsKey(ctx, id)
}

func (s *SettingsController) NewUserSettings(ctx context.Context, dto *dto.UserSettingsDto) error {
	return s.settingsService.NewUserSettings(ctx, dto)
}

func (s *SettingsController) UserSettingsKeyList(ctx context.Context, params *dto.UserKeyParams) (*vo.PageVo, error) {
	return s.settingsService.UserSettingsKeyList(ctx, params)
}
