package controller

import (
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/mapper"
	"linkany/pkg/log"
)

type InviteController struct {
	logger         *log.Logger
	invititeMapper mapper.InviteInterface
}

func NewInviteController(inviteMapper mapper.InviteInterface, logger *log.Logger) *InviteController {
	return &InviteController{invititeMapper: inviteMapper, logger: logger}
}

func (i *InviteController) Invite(dto *dto.InviteDto) error {
	return i.invititeMapper.Invite(dto)
}

func (i *InviteController) Get(userId, email string) (*entity.Invitations, error) {
	i.logger.Verbosef("Get invitation by userId: %s, email: %s", userId, email)
	return i.invititeMapper.Get(userId, email)
}

func (i *InviteController) Update(dto *dto.InviteDto) error {
	return i.invititeMapper.Update(dto)
}

func (i *InviteController) List(params *mapper.QueryParams) ([]*entity.Invitations, error) {
	return i.invititeMapper.List(params)
}
