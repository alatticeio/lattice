package controller

import (
	"context"
	"wireflow/internal/config"
	"wireflow/management/model"
	"wireflow/management/resource"
	"wireflow/management/service"
)

type TeamController interface {
	OnboardExternalUser(ctx context.Context, userId, extEmail string) (*model.User, error)
}

type teamController struct {
	teamService service.TeamService
}

func (t teamController) OnboardExternalUser(ctx context.Context, userId, extEmail string) (*model.User, error) {
	return t.teamService.OnboardExternalUser(ctx, userId, extEmail)
}

func NewTeamController(client *resource.Client, config *config.Config) TeamController {
	return &teamController{
		teamService: service.NewTeamService(client, config),
	}
}
