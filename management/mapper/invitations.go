package mapper

import (
	"linkany/management/dto"
	"linkany/management/entity"
	"time"
)

type InviteInterface interface {
	// Invite a user join network
	Invite(dto *dto.InviteDto) error
	Get(userId, email string) (*entity.Invitations, error)
	Update(dto *dto.InviteDto) error
	List(paras *QueryParams) ([]*entity.Invitations, error)
}

type InvitationMapper struct {
	*DatabaseService
}

func NewInviteMapper(dataBaseService *DatabaseService) *InvitationMapper {
	return &InvitationMapper{dataBaseService}
}

func (im *InvitationMapper) Invite(dto *dto.InviteDto) error {
	invitate := entity.Invitations{
		InvitationId: dto.InvitationId,
		InviterId:    dto.InviterId,
		MobilePhone:  "",
		Email:        dto.Email,
		AcceptStatus: entity.NewInvite,
		InvitedAt:    time.Now(),
	}
	im.Create(&invitate)
	return nil
}

func (im *InvitationMapper) Get(userId, email string) (*entity.Invitations, error) {
	var inv entity.Invitations
	if err := im.Where("invitation_id = ? AND email = ?", userId, email).First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

func (im *InvitationMapper) Update(dto *dto.InviteDto) error {
	var inv entity.Invitations
	if err := im.Where("invitation_id = ? AND email = ?", dto.InvitationId, dto.Email).First(&inv).Error; err != nil {
		return err
	}
	inv.AcceptStatus = entity.Accept
	inv.AcceptAt = time.Now()
	im.Save(&inv)
	return nil
}

func (im *InvitationMapper) List(params *QueryParams) ([]*entity.Invitations, error) {
	var invs []*entity.Invitations
	if err := im.Where("invitation_id = ?", params.UserId).Find(&invs).Error; err != nil {
		return nil, err
	}
	return invs, nil
}
