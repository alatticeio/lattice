package dto

import (
	"linkany/management/vo"
)

type QueryParams struct {
	vo.PageModel
	Keyword *string `json:"keyword" form:"keyword"`
	Name    *string `json:"name" form:"name"`
	PubKey  *string `json:"pubKey" form:"pubKey"`
	UserId  *string `json:"userId" form:"userId"`
	Status  *int
}

func (qp *QueryParams) Generate() []*KeyValue {
	var result []*KeyValue

	if qp.Name != nil {
		result = append(result, newKeyValue("name", *qp.Name))
	}

	if qp.PubKey != nil {
		result = append(result, newKeyValue("pub_key", *qp.PubKey))
	}

	if qp.UserId != nil {
		result = append(result, newKeyValue("user_id", *qp.UserId))
	}

	if qp.Status != nil {
		result = append(result, newKeyValue("status", *qp.Status))
	}

	return result
}

// NetworkMapInterface user's network map
type NetworkMapInterface interface {
	GetNetworkMap(pubKey, userId string) (*vo.NetworkMap, error)
}
