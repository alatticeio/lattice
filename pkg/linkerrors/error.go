package linkerrors

import "errors"

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrorServerInterval = errors.New("interval server error")
	ErrInvalidOffer     = errors.New("invalid offer")
	ErrChannelNotExists = errors.New("channel not exists")
	ErrClientCanceled   = errors.New("client canceled")
	ErrClientClosed     = errors.New("client closed")
	ErrProberNotFound   = errors.New("prober not found")
	ErrPasswordRequired = errors.New("password required")
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrAgentNotFound    = errors.New("agent not found")
	ErrProbeFailed      = errors.New("probe connect failed, need check the network you are in")
	ErrorNotSameGroup   = errors.New("not in the same group")
	ErrInvitationExists = errors.New("invitation already exists")
)
