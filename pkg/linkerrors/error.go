package linkerrors

import "errors"

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrorServerInterval = errors.New("interval server error")
	ErrInvalidOffer     = errors.New("invalid offer")
	ErrChannelNotExists = errors.New("channel not exists")
	ErrorClientCanceled = errors.New("client canceled")
	ErrorClientClosed   = errors.New("client closed")
)
