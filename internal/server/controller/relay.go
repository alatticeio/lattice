package controller

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/resource"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/internal/server/vo"
)

// RelayController handles HTTP-layer relay management operations.
type RelayController interface {
	List(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.RelayVo], error)
	Create(ctx context.Context, req *dto.RelayDto) (*vo.RelayVo, error)
	Update(ctx context.Context, id string, req *dto.RelayDto) (*vo.RelayVo, error)
	Delete(ctx context.Context, id string) error
	Test(ctx context.Context, id string) (*vo.RelayTestVo, error)
}

type relayController struct {
	svc service.RelayService
}

func (c *relayController) List(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.RelayVo], error) {
	return c.svc.List(ctx, pageParam)
}

func (c *relayController) Create(ctx context.Context, req *dto.RelayDto) (*vo.RelayVo, error) {
	return c.svc.Create(ctx, req)
}

func (c *relayController) Update(ctx context.Context, id string, req *dto.RelayDto) (*vo.RelayVo, error) {
	return c.svc.Update(ctx, id, req)
}

func (c *relayController) Delete(ctx context.Context, id string) error {
	return c.svc.Delete(ctx, id)
}

func (c *relayController) Test(ctx context.Context, id string) (*vo.RelayTestVo, error) {
	return c.svc.Test(ctx, id)
}

// NewRelayController constructs a RelayController.
func NewRelayController(c *resource.Client, st store.Store) RelayController {
	return &relayController{
		svc: service.NewRelayService(c, st),
	}
}
