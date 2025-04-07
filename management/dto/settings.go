package dto

import (
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"

	"gorm.io/gorm"
)

type AppKeyDto struct {
	gorm.Model
	AppKey string `json:"appKey"`
	Status entity.ActiveStatus
}

type AppKeyParams struct {
	vo.PageModel
	UserId uint `json:"userId" form:"userId"`
}

func (p *AppKeyParams) Generate() []*utils.KeyValue {
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
