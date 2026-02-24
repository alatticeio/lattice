package controller

import (
	"context"
	"wireflow/management/resource"
	"wireflow/management/service"
)

type TokenController interface {

	// Create token for web
	Create(ctx context.Context) error

	Delete(ctx context.Context, token string) error
}

type tokenController struct {
	tokenService service.TokenService
}

func (t *tokenController) Delete(ctx context.Context, token string) error {
	return t.tokenService.Delete(ctx, token)
}

func (t *tokenController) Create(ctx context.Context) error {
	return t.tokenService.Create(ctx)
}

func NewTokenController(client *resource.Client) TokenController {
	return &tokenController{
		tokenService: service.NewTokenService(client),
	}
}
