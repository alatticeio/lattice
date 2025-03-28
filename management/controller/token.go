package controller

import (
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/pkg/log"
)

type TokenController struct {
	logger       *log.Logger
	tokenService service.TokenService
}

func NewTokenController(db *service.DatabaseService) *TokenController {
	return &TokenController{
		logger:       log.NewLogger(log.Loglevel, "token-controller"),
		tokenService: service.NewTokenService(db),
	}
}

func (t *TokenController) Generate(username, password string) (string, error) {
	return t.tokenService.Generate(username, password)
}

func (t *TokenController) Verify(username, password string) (bool, *entity.User, error) {
	return t.tokenService.Verify(username, password)
}

func (t *TokenController) Parse(token string) (*entity.User, error) {
	return t.tokenService.Parse(token)
}
