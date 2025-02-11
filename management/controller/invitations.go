package controller

import (
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/mapper"
)

type InvititationController struct {
	invititationMapper mapper.InvitationInterface
}

func NewInvititationController(invititationMapper mapper.InvitationInterface) *InvititationController {
	return &InvititationController{invititationMapper: invititationMapper}
}

func (i *InvititationController) Invite(dto *dto.InviteDto) error {
	return i.invititationMapper.Invite(dto)
}

func (i *InvititationController) Get(userId, email string) (*entity.Invitations, error) {
	return i.invititationMapper.Get(userId, email)
}

func (i *InvititationController) Update(dto *dto.InviteDto) error {
	return i.invititationMapper.Update(dto)
}

func (i *InvititationController) List(params *mapper.QueryParams) ([]*entity.Invitations, error) {
	return i.invititationMapper.List(params)
}
