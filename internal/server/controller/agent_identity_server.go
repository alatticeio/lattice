package controller

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
)

// AgentIdentityServerController defines the AgentIdentity HTTP operations.
type AgentIdentityServerController interface {
	List(ctx context.Context, tenantID string) ([]*models.AgentIdentity, error)
	Get(ctx context.Context, id string) (*models.AgentIdentity, error)
	Create(ctx context.Context, tenantID string, req service.CreateAgentIdentityRequest) (*models.AgentIdentity, error)
	Update(ctx context.Context, id string, req service.CreateAgentIdentityRequest) (*models.AgentIdentity, error)
	Delete(ctx context.Context, id string) error
}

type agentIdentityServerController struct {
	svc   *service.AgentIdentityService
	store store.Store
}

// NewAgentIdentityServerController creates a new AgentIdentityServerController.
func NewAgentIdentityServerController(st store.Store) AgentIdentityServerController {
	return &agentIdentityServerController{
		svc:   service.NewAgentIdentityService(st),
		store: st,
	}
}

func (c *agentIdentityServerController) List(ctx context.Context, tenantID string) ([]*models.AgentIdentity, error) {
	return c.svc.List(ctx, tenantID)
}

func (c *agentIdentityServerController) Get(ctx context.Context, id string) (*models.AgentIdentity, error) {
	return c.svc.Get(ctx, id)
}

func (c *agentIdentityServerController) Create(ctx context.Context, tenantID string, req service.CreateAgentIdentityRequest) (*models.AgentIdentity, error) {
	return c.svc.Create(ctx, tenantID, req)
}

func (c *agentIdentityServerController) Update(ctx context.Context, id string, req service.CreateAgentIdentityRequest) (*models.AgentIdentity, error) {
	return c.svc.Update(ctx, id, req)
}

func (c *agentIdentityServerController) Delete(ctx context.Context, id string) error {
	return c.svc.Delete(ctx, id)
}
