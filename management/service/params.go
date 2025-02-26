package service

import (
	"linkany/management/entity"
)

type QueryParams struct {
	PubKey   *string
	UserId   *string
	Status   *int
	Total    *int
	PageNo   *int
	PageSize *int

	filters []*kv
}

type kv struct {
	Key   string
	Value interface{}
}

func (qp *QueryParams) Generate() []*kv {
	var result []*kv

	if qp.PubKey != nil {
		v := &kv{
			Key:   "public_key",
			Value: qp.PubKey,
		}

		result = append(result, v)
	}

	if qp.UserId != nil {
		v := &kv{
			Key:   "user_id",
			Value: qp.UserId,
		}

		result = append(result, v)
	}

	if qp.Status != nil {
		v := &kv{
			Key:   "status",
			Value: qp.Status,
		}

		result = append(result, v)
	}

	return result
}

// NetworkMapInterface user's network map
type NetworkMapInterface interface {
	GetNetworkMap(pubKey, userId string) (*entity.NetworkMap, error)
}
