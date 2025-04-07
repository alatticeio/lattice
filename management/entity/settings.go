package entity

import (
	"linkany/management/utils"

	"gorm.io/gorm"
)

type AppKey struct {
	gorm.Model
	OrderId uint
	UserId  uint
	AppKey  string
	Status  ActiveStatus
}

type ActiveStatus int

const (
	Active ActiveStatus = iota
	Inactive
	Frozen
)

func (ak ActiveStatus) String() string {
	switch ak {
	case Active:
		return "active"
	case Inactive:
		return "inactive"
	case Frozen:
		return "frozen"
	}
	return ""
}

func (AppKey) TableName() string {
	return "la_user_app_key"
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
