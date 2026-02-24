package model

// UserConfig is a struct that contains invitation configuration for a user
type UserConfig struct {
	UserID      uint64 // invitation user id
	InviteLimit int    // free user can only invite 5 users
	PeersLimit  int    // free user can only have 100 peers total
}

type NodeLimit struct {
	UserId             uint64
	PlanType           string
	NodeLimit          uint
	NodeRegisterdCount uint
	NodeFreeCount      uint
}

func (uc *UserConfig) TableName() string {
	return "la_user_config"
}
