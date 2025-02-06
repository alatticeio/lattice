package dto

// UserDto is a data transfer object for User entity
type UserDto struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// PeerDto is a data transfer object for Peer entity
type PeerDto struct {
	PubKey string
	Online int
}

// PlanDto is a data transfer object for Plan entity
type PlanDto struct {
}

// SupportDto is a data transfer object for Support entity
type SupportDto struct {
}

type InviteDto struct {
	InvitationId int64
	InviterId    int64
	Email        string
	Network      string // 192.168.0.0/24
}
