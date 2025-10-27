package http

import "github.com/gin-gonic/gin"

// pushFilter is used to push the request to all connected grpc clients

// CallbackContext is the context for the callback function
type CallbackContext struct {
	Path     string
	Method   string
	Action   Action
	Status   int
	GroupID  string
	ClientID string
}

type CallbackFunc interface {
	Callback(ctx *CallbackContext) error
}

// WithCallback 用来统一处理请求完的推送信息
func (s *Server) withCallback() gin.HandlerFunc {
	return func(c *gin.Context) {
		actionString := c.Request.Header.Get("action")
		c.Next()

		if !IsValidAction(actionString) {
			return
		}

		s.Callback(&CallbackContext{
			Status: c.Writer.Status(),
			Action: ActionFromString(actionString),
		})
	}
}
