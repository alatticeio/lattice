package controller

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/internal/server/vo"
)

type InvitationController interface {
	Create(ctx context.Context, workspaceID, inviterID, email string, role dto.WorkspaceRole) (*models.WorkspaceInvitation, error)
	Preview(ctx context.Context, token string) (*vo.InvitePreviewVo, error)
	Accept(ctx context.Context, token, userID string) error
	RegisterAndAccept(ctx context.Context, token, username, password string) (string, error)
	Revoke(ctx context.Context, callerID, invitationID string) error
	List(ctx context.Context, workspaceID string) ([]*models.WorkspaceInvitation, error)
}

type invitationController struct {
	svc service.InvitationService
}

func NewInvitationController(st store.Store) InvitationController {
	return &invitationController{svc: service.NewInvitationService(st)}
}

func (c *invitationController) Create(ctx context.Context, workspaceID, inviterID, email string, role dto.WorkspaceRole) (*models.WorkspaceInvitation, error) {
	return c.svc.Create(ctx, workspaceID, inviterID, email, role)
}

func (c *invitationController) Preview(ctx context.Context, token string) (*vo.InvitePreviewVo, error) {
	return c.svc.Preview(ctx, token)
}

func (c *invitationController) Accept(ctx context.Context, token, userID string) error {
	return c.svc.Accept(ctx, token, userID)
}

func (c *invitationController) RegisterAndAccept(ctx context.Context, token, username, password string) (string, error) {
	return c.svc.RegisterAndAccept(ctx, token, username, password)
}

func (c *invitationController) Revoke(ctx context.Context, callerID, invitationID string) error {
	return c.svc.Revoke(ctx, callerID, invitationID)
}

func (c *invitationController) List(ctx context.Context, workspaceID string) ([]*models.WorkspaceInvitation, error) {
	return c.svc.List(ctx, workspaceID)
}
