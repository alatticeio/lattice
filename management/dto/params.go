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
	Page    *int    `json:"page" form:"page"`
	Size    *int    `json:"size" form:"size"`
	Status  *int
}

func (p *QueryParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if p.Name != nil {
		result = append(result, utils.NewKeyValue("name", *p.Name))
	}

	if p.PubKey != nil {
		result = append(result, utils.NewKeyValue("public_key", *p.PubKey))
	}

	if p.UserId != "" {
		result = append(result, utils.NewKeyValue("user_id", p.UserId))
	}

	if p.Status != nil {
		result = append(result, utils.NewKeyValue("status", *p.Status))
	}

	// if over 50, set to default
	if p.Size != nil && *p.Size > 50 {
		*p.Size = utils.PageSize
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

	return result
}

// NetworkMapInterface user's network map
type NetworkMapInterface interface {
	GetNetworkMap(pubKey, userId string) (*vo.NetworkMap, error)
}
