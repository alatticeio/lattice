package dto

const (
	// PageNo default page number
	PageNo = 1
	// PageSize default page size
	PageSize = 10
)

type PageModel struct {
	Total    int
	PageNo   int
	PageSize int
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
