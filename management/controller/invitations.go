package controller

import (
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/pkg/log"
)

type InviteController struct {
	logger      *log.Logger
	userService service.UserService
}

func NewInviteController(inviteMapper service.UserService) *InviteController {
	return &InviteController{userService: inviteMapper, logger: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "invite-controller"))}
}

func (i *InviteController) Invite(dto *dto.InviteDto) error {
	return i.userService.Invite(dto)
}

func (i *InviteController) Get(userId, email string) (*entity.Invitation, error) {
	i.logger.Verbosef("Get invitation by userId: %s, email: %s", userId, email)
	return i.userService.GetInvitation(userId, email)
}

func (i *InviteController) Update(dto *dto.InviteDto) error {
	return i.userService.UpdateInvitation(dto)
}

func (i *InviteController) List(params *service.QueryParams) ([]*entity.Invitation, error) {
	return i.userService.ListInvitations(params)
}
