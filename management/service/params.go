package service

import (
	"linkany/management/dto"
	"linkany/management/entity"
)

type QueryParams struct {
	PubKey   *string
	UserId   *string
	Status   *int
	Total    *int
	PageNo   *int
	PageSize *int
}

func (qp *QueryParams) Generate() []*dto.KeyValue {
	var result []*dto.KeyValue

	if qp.PubKey != nil {
		result = append(result, dto.NewKV("pub_key", qp.PubKey))
	}

	if qp.UserId != nil {
		result = append(result, dto.NewKV("user_id", qp.UserId))
	}

	if qp.Status != nil {
		result = append(result, dto.NewKV("status", qp.Status))
	}

	return result
}

// NetworkMapInterface user's network map
type NetworkMapInterface interface {
	GetNetworkMap(pubKey, userId string) (*entity.NetworkMap, error)
}
