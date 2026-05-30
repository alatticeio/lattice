package controller

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
)

// PeerIdentityController defines the PeerIdentity operations.
type PeerIdentityController interface {
	List(ctx context.Context, networkID string) ([]*models.PeerIdentity, error)
	Get(ctx context.Context, id string) (*models.PeerIdentity, error)
	Create(ctx context.Context, networkID string, req service.CreatePeerIdentityRequest) (*models.PeerIdentity, error)
	Update(ctx context.Context, id string, req service.CreatePeerIdentityRequest) (*models.PeerIdentity, error)
	Delete(ctx context.Context, id string) error
}

type peerIdentityController struct {
	svc   *service.PeerIdentityService
	store store.Store
}

// NewPeerIdentityController creates a new PeerIdentityController.
func NewPeerIdentityController(st store.Store) PeerIdentityController {
	return &peerIdentityController{
		svc:   service.NewPeerIdentityService(st),
		store: st,
	}
}

func (c *peerIdentityController) List(ctx context.Context, networkID string) ([]*models.PeerIdentity, error) {
	return c.svc.List(ctx, networkID)
}

func (c *peerIdentityController) Get(ctx context.Context, id string) (*models.PeerIdentity, error) {
	return c.svc.Get(ctx, id)
}

func (c *peerIdentityController) Create(ctx context.Context, networkID string, req service.CreatePeerIdentityRequest) (*models.PeerIdentity, error) {
	return c.svc.Create(ctx, networkID, req)
}

func (c *peerIdentityController) Update(ctx context.Context, id string, req service.CreatePeerIdentityRequest) (*models.PeerIdentity, error) {
	return c.svc.Update(ctx, id, req)
}

func (c *peerIdentityController) Delete(ctx context.Context, id string) error {
	return c.svc.Delete(ctx, id)
}
