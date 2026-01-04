package entity

type AppKey struct {
	Model
	OrderId uint64
	UserId  uint64
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
	Model
	AppKey     string
	PlanType   string
	NodeLimit  uint64
	NodeFree   uint64
	GroupLimit uint64
	GroupFree  uint64
}

func (UserSettings) TableName() string {
	return "la_user_settings"
}
