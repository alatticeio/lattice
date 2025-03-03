package dto

import (
	"linkany/management/entity"
)

type QueryParams struct {
	PageModel
	PubKey   *string
	UserId   *string
	Status   *int
	Total    *int
	PageNo   *int
	PageSize *int
}

func (qp *QueryParams) Generate() []*KeyValue {
	var result []*KeyValue

	if qp.PubKey != nil {
		result = append(result, newKeyValue("pub_key", qp.PubKey))
	}

	if qp.UserId != nil {
		result = append(result, newKeyValue("user_id", qp.UserId))
	}

	if qp.Status != nil {
		result = append(result, newKeyValue("status", qp.Status))
	}

	return result
}

// NetworkMapInterface user's network map
type NetworkMapInterface interface {
	GetNetworkMap(pubKey, userId string) (*entity.NetworkMap, error)
}
