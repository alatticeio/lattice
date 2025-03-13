package utils

const (
	// PageNo default page number
	PageNo = 1
	// PageSize default page size
	PageSize = 1000
)

type AcceptType string

const (
	ACCEPT AcceptType = "accepted"
	REJECT AcceptType = "rejected"
)

type KeyValue struct {
	Key   string
	Value interface{}
}

func NewKeyValue(k string, v interface{}) *KeyValue {
	return &KeyValue{
		Key:   k,
		Value: v,
	}
}

type ParamBuilder interface {
	Generate() []*KeyValue
}
