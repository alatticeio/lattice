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
	natsURL, err := c.store.SystemConfig().Get(ctx, models.ConfigKeyNatsURL)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	stunURL, err := c.store.SystemConfig().Get(ctx, models.ConfigKeyStunURL)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &dto.PlatformSettingsResponse{NatsURL: natsURL, StunURL: stunURL}, nil
}

func (c *platformController) UpdateSettings(ctx context.Context, req dto.PlatformSettingsRequest) error {
	if err := c.store.SystemConfig().Set(ctx, models.ConfigKeyNatsURL, req.NatsURL); err != nil {
		return err
	}
	return c.store.SystemConfig().Set(ctx, models.ConfigKeyStunURL, req.StunURL)
}
