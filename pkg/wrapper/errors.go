package wrapper

import "errors"

var (
	errInvalidIPAddress    = errors.New("invalid ip address")
	errInvalidAddress      = errors.New("invalid address")
	errFailedToCastUDPAddr = errors.New("failed to cast net.Addr to net.UDPAddr")
)
