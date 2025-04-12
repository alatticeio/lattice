package linkerrors

import "errors"

var (
	ErrAppKeyRequired      = errors.New("app key is required")
	ErrInvalidToken        = errors.New("invalid token")
	ErrorServerInterval    = errors.New("interval server error")
	ErrInvalidOffer        = errors.New("invalid offer")
	ErrChannelNotExists    = errors.New("channel not exists")
	ErrClientCanceled      = errors.New("client canceled")
	ErrClientClosed        = errors.New("client closed")
	ErrProberNotFound      = errors.New("prober not found")
	ErrPasswordRequired    = errors.New("password required")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidPassword     = errors.New("invalid password")
	ErrAgentNotFound       = errors.New("agent not found")
	ErrProbeFailed         = errors.New("probe connect failed, need check the network you are in")
	ErrorNotSameGroup      = errors.New("not in the same group")
	ErrInvitationExists    = errors.New("invitation already exists")
	ErrNoAccessPermissions = errors.New("no permissions to access this resource,please contact to resource owner")

	ErrDeleteSharedGroup = errors.New("cannot delete shared group, please contact the owner")
	ErrDeleteSharedNode  = errors.New("cannot delete shared node, please contact the owner")
)
