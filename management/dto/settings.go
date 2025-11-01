package dto

import (
	"wireflow/management/entity"
	"wireflow/management/vo"
	utils2 "wireflow/pkg/utils"
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

func (p *AppKeyParams) Generate() []*utils2.KeyValue {
	var result []*utils2.KeyValue

	if p.UserId != 0 {
		result = append(result, utils2.NewKeyValue("user_id", p.UserId))
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
	FromDate   utils2.NullTime
	EndDate    utils2.NullTime
}
