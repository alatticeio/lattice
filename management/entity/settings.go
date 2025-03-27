package entity

import (
	"gorm.io/gorm"
	"linkany/management/utils"
)

type UserSettingsKey struct {
	gorm.Model
	UserId  uint
	UserKey string
}

func (UserSettingsKey) TableName() string {
	return "la_user_settings_key"
}

type UserSettings struct {
	gorm.Model
	AppKey     string
	PlanType   string
	NodeLimit  uint
	NodeFree   uint
	GroupLimit uint
	GroupFree  uint
	FromDate   utils.NullTime
	EndDate    utils.NullTime
}

func (UserSettings) TableName() string {
	return "la_user_settings"
}
