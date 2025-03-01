package http

import "net/http"

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func NewResponse(code int, msg string, data interface{}) *Response {
	return &Response{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}

func WriteOK(fn func(code int, obj any), data interface{}) {
	fn(http.StatusOK, NewResponse(http.StatusOK, "success", data))
}

func WriteError(fn func(code int, obj any), msg string) {
	fn(http.StatusOK, NewResponse(http.StatusInternalServerError, msg, nil))
}

func WriteBadRequest(fn func(code int, obj any), msg string) {
	fn(http.StatusOK, NewResponse(http.StatusBadRequest, msg, nil))
}
