package dto

import (
	"linkany/management/utils"
	"linkany/management/vo"
)

type UserSettingsKeyDto struct {
	UserSettingsKey string `json:"userKey"`
}

type UserKeyParams struct {
	vo.PageModel
	UserId uint `json:"userId" form:"userId"`
}

func (p *UserKeyParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.UserId != 0 {
		result = append(result, utils.NewKeyValue("user_id", p.UserId))
	}

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}
	return result
}

type UserSettingsDto struct {
	AppKey     string
	PlanType   string
	NodeLimit  uint
	NodeFree   uint
	GroupLimit uint
	GroupFree  uint
	FromDate   utils.NullTime
	EndDate    utils.NullTime
}
