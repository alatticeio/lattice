package client

import "net/http"

// HttpResponse will convert json to real type
type HttpResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
	Data    T      `json:"data"`
}

func Unauthorized() (int, *HttpResponse[any]) {
	return http.StatusUnauthorized, &HttpResponse[any]{
		Code:    http.StatusUnauthorized,
		Message: "Unauthorized",
	}
}

func InternalServerError(err error) (int, *HttpResponse[any]) {
	return http.StatusUnauthorized, &HttpResponse[any]{
		Code:    http.StatusInternalServerError,
		Message: err.Error(),
	}
}

func BadRequest(err error) (int, *HttpResponse[any]) {
	return http.StatusBadRequest, &HttpResponse[any]{
		Code:    http.StatusBadRequest,
		Message: err.Error(),
	}
}

func Success(data interface{}) (int, *HttpResponse[any]) {
	return http.StatusOK, &HttpResponse[any]{
		Code:    http.StatusOK,
		Message: "success",
		Data:    data,
	}
}
