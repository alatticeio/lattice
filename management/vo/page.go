package vo

import "linkany/management/dto"

type PageVo struct {
	Data interface{} `json:"data"`
	dto.PageModel
}
