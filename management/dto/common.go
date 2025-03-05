package dto

const (
	// PageNo default page number
	PageNo = 1
	// PageSize default page size
	PageSize = 10
)

// PageModel is a data transfer object for pagination, add 'form' is used to bind data from request
// like c.ShouldBindQuery using.
type PageModel struct {
	Total   int64 `json:"total" form:"total"`
	Page    int   `json:"page" form:"page"`
	Current int   `json:"current" form:"current""`
	Size    int   `json:"size" form:"size"`
}

type AcceptType string

const (
	ACCEPT AcceptType = "accepted"
	REJECT AcceptType = "rejected"
)

type KeyValue struct {
	Key   string
	Value interface{}
}

func newKeyValue(k string, v interface{}) *KeyValue {
	return &KeyValue{
		Key:   k,
		Value: v,
	}
}

type ParamBuilder interface {
	Generate() []*KeyValue
}
