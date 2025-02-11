package mapper

import (
	"linkany/management/dto"
	"linkany/management/entity"
)

// UserInterface is an interface for user mapper
type UserInterface interface {
	Login(u *dto.UserDto) (*entity.Token, error)
	Register(e *dto.UserDto) (*entity.User, error)

	//Get returns a user by token
	Get(token string) (*entity.User, error)
}

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

// PlanInterface is an interface for plan mapper
type PlanInterface interface {
	// List returns a list of plans
	List() ([]*entity.Plan, error)
	Get() (*entity.Plan, error)
	Page() (*entity.Plan, error)
}

type SupportInterface interface {
	// List returns a list of supports
	List() ([]*entity.Support, error)
	Get() (*entity.Support, error)
	Page() (*entity.Support, error)
	Create(e *dto.SupportDto) (*entity.Support, error)
}

// NetworkMapInterface user's network map
type NetworkMapInterface interface {
	GetNetworkMap(pubKey, userId string) (*entity.NetworkMap, error)
}
