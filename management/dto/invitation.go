package dto

import (
	"wireflow/management/vo"
	"wireflow/pkg/utils"
)

type InvitationParams struct {
	vo.PageModel
	UserId      *string
	Email       *string
	MobilePhone *string
	Type        *InviteType
	Status      *utils.AcceptType
}

type InviterParams struct {
	vo.PageModel
	InviterId uint64
	Type      *InviteType
	Status    *utils.AcceptType
}

type InviteType string

var (
	INVITE  InviteType = "invite"  // invite to others
	INVITED InviteType = "invited" // other invite to
)

func (p *InviterParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.InviterId != 0 {
		result = append(result, utils.NewKeyValue("inviter_id", p.InviterId))
	}

	if p.Type != nil {
		result = append(result, utils.NewKeyValue("type", p.Type))
	}

	if p.Status != nil {
		result = append(result, utils.NewKeyValue("status", p.Status))
	}

	return result
}

func (p *InvitationParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.UserId != nil {
		result = append(result, utils.NewKeyValue("user_id", p.UserId))
	}

	if p.Type != nil {
		result = append(result, utils.NewKeyValue("Type", p.Type))
	}

	if p.Status != nil {
		result = append(result, utils.NewKeyValue("status", p.Status))
	}

	return result
}
