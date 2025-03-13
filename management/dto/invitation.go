package dto

import (
	"linkany/management/utils"
	"linkany/management/vo"
)

type InvitationParams struct {
	vo.PageModel
	UserId      *string
	Email       *string
	MobilePhone *string
	Type        *InviteType
	Status      *utils.AcceptType
}

type InviteType string

var (
	INVITE  InviteType = "invite"  // invite to others
	INVITED InviteType = "invited" // other invite to
)

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

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}

	return result
}
