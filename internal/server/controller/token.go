package controller

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/resource"
	"github.com/alatticeio/lattice/internal/server/service"
)

type TokenController interface {
	Create(ctx context.Context) (string, error)
	Delete(ctx context.Context, token string) error
}

type tokenController struct {
	tokenService service.TokenService
}

func (t *tokenController) Delete(ctx context.Context, token string) error {
	return t.tokenService.Delete(ctx, token)
}

func (t *tokenController) Create(ctx context.Context) (string, error) {
	return t.tokenService.Create(ctx)
}

func NewTokenController(client *resource.Client, st store.Store) TokenController {
	return &tokenController{
		tokenService: service.NewTokenService(client, st),
	}
}
