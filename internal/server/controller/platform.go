package controller

import (
	"context"
	"errors"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
)

type PlatformController interface {
	GetSettings(ctx context.Context) (*dto.PlatformSettingsResponse, error)
	UpdateSettings(ctx context.Context, req dto.PlatformSettingsRequest) error
}

type platformController struct {
	store store.Store
}

func NewPlatformController(st store.Store) PlatformController {
	return &platformController{store: st}
}

func (c *platformController) GetSettings(ctx context.Context) (*dto.PlatformSettingsResponse, error) {
	val, err := c.store.SystemConfig().Get(ctx, models.ConfigKeyNatsURL)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &dto.PlatformSettingsResponse{}, nil
		}
		return nil, err
	}
	return &dto.PlatformSettingsResponse{NatsURL: val}, nil
}

func (c *platformController) UpdateSettings(ctx context.Context, req dto.PlatformSettingsRequest) error {
	return c.store.SystemConfig().Set(ctx, models.ConfigKeyNatsURL, req.NatsURL)
}
