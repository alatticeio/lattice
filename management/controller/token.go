package controller

import (
	"context"
	"gorm.io/gorm"
	"wireflow/management/entity"
	"wireflow/management/service"
	"wireflow/pkg/log"
)

type TokenController struct {
	logger       *log.Logger
	tokenService service.TokenService
}

func NewTokenController(db *gorm.DB) *TokenController {
	return &TokenController{
		logger:       log.NewLogger(log.Loglevel, "token-controller"),
		tokenService: service.NewTokenService(db),
	}
}

func (t *TokenController) Generate(username, password string) (string, error) {
	return t.tokenService.Generate(username, password)
}

func (t *TokenController) Verify(ctx context.Context, username, password string) (bool, *entity.User, error) {
	return t.tokenService.Verify(ctx, username, password)
}

func (t *TokenController) Parse(token string) (*entity.User, error) {
	return t.tokenService.Parse(token)
}
