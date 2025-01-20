package linkerrors

import "errors"

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrorServerInterval = errors.New("interval server error")
	ErrInvalidOffer     = errors.New("invalid offer")
)
