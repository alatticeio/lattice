package client

// HttpResponse will convert json to real type
type HttpResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
	Data    T      `json:"data"`
}
