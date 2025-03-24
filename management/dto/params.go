package dto

import (
	"linkany/management/utils"
	"linkany/management/vo"
)

type QueryParams struct {
	vo.PageModel
	Keyword *string `json:"keyword" form:"keyword"`
	Name    *string `json:"name" form:"name"`
	PubKey  *string `json:"pubKey" form:"pubKey"`
	UserId  string  `json:"userId" form:"userId"`
	Page    int     `json:"page" form:"page"`
	Size    int     `json:"size" form:"size"`
	Status  *int
}

func (qp *QueryParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if qp.Name != nil {
		result = append(result, utils.NewKeyValue("name", *qp.Name))
	}

	if qp.PubKey != nil {
		result = append(result, utils.NewKeyValue("pub_key", *qp.PubKey))
	}

	if qp.UserId != "" {
		result = append(result, utils.NewKeyValue("user_id", qp.UserId))
	}

	if qp.Status != nil {
		result = append(result, utils.NewKeyValue("status", *qp.Status))
	}

	return result
}

type PermissionParams struct {
	vo.PageModel
	Name string `json:"name" form:"name"`
}

func (p *PermissionParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.Name != "" {
		result = append(result, utils.NewKeyValue("name", p.Name))
	}

	if p.Page == 0 {
		p.Page = utils.PageNo
	}

	if p.Size == 0 {
		p.Size = utils.PageSize
	}

	return result
}

// NetworkMapInterface user's network map
type NetworkMapInterface interface {
	GetNetworkMap(pubKey, userId string) (*vo.NetworkMap, error)
}
