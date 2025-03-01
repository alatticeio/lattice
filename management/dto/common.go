package dto

const (
	// PageNo default page number
	PageNo = 1
	// PageSize default page size
	PageSize = 10
)

type PageModel struct {
	Total    int64 `json:"total"`
	PageNo   int   `json:"pageNo"`
	Current  int   `json:"current"`
	PageSize int   `json:"pageSize"`
}

type AcceptType string

const (
	ACCEPT AcceptType = "accepted"
	REJECT AcceptType = "rejected"
)

type Condition string

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
