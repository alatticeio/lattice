package entity

import "gorm.io/gorm"

// UserConfig is a struct that contains invitation configuration for a user
type UserConfig struct {
	gorm.Model
	UserID      int64 // invitation user id
	InviteLimit int   // free user can only invite 5 users
	PeersLimit  int   // free user can only have 100 peers total
}

type AppKey struct {
	gorm.Model
	UserId uint
	AppKey string
}

func (AppKey) TableName() string {
	return "la_app_key"
}

type NodeLimit struct {
	gorm.Model
	UserId             uint
	PlanType           string
	NodeLimit          uint
	NodeRegisterdCount uint
	NodeFreeCount      uint
}

func (uc *UserConfig) TableName() string {
	return "la_user_config"
}
