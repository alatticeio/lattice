package dto

type InvitationParams struct {
	PageModel
	UserId      *string
	Email       *string
	MobilePhone *string
	Type        *InviteType
	Status      *AcceptType
}

type InviteType string

var (
	INVITE  InviteType = "invite"  // invite to others
	INVITED InviteType = "invited" // other invite to
)

func (p *InvitationParams) Generate() []*KeyValue {
	var result []*KeyValue

	if p.UserId != nil {
		result = append(result, newKeyValue("user_id", p.UserId))
	}

	if p.Type != nil {
		result = append(result, newKeyValue("Type", p.Type))
	}

	if p.Status != nil {
		result = append(result, newKeyValue("status", p.Status))
	}

	if p.Page == 0 {
		p.Page = PageNo
	}

	if p.Size == 0 {
		p.Size = PageSize
	}

	return result
}
