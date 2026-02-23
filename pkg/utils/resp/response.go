package resp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

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

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, NewResponse(http.StatusOK, "success", data))
}

func Error(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, NewResponse(http.StatusInternalServerError, msg, nil))
}

func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, NewResponse(http.StatusForbidden, msg, nil))
}

func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, NewResponse(http.StatusUnauthorized, msg, nil))
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, NewResponse(http.StatusBadRequest, msg, nil))
}
