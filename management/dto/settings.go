package dto

import (
	"wireflow/management/entity"
	"wireflow/management/utils"
	"wireflow/management/vo"
)

type AppKeyDto struct {
	ID     uint64
	AppKey string `json:"appKey"`
	Status entity.ActiveStatus
}

type AppKeyParams struct {
	vo.PageModel
	UserId uint64 `json:"userId" form:"userId"`
}

func (p *AppKeyParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.UserId != 0 {
		result = append(result, utils.NewKeyValue("user_id", p.UserId))
	}

	return result
}

type UserSettingsDto struct {
	AppKey     string
	PlanType   string
	NodeLimit  uint64
	NodeFree   uint64
	GroupLimit uint64
	GroupFree  uint64
	FromDate   utils.NullTime
	EndDate    utils.NullTime
}
