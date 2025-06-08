package internal

type ConnectionState int

const (
	ConnectionStateNew ConnectionState = iota
	ConnectionStateChecking
	ConnectionStateFailed
	ConnectionStateConnected
	ConnectionStateDisconnected
)

func (c ConnectionState) String() string {
	switch c {
	case ConnectionStateNew:
		return "New"
	case ConnectionStateChecking:
		return "Checking"
	case ConnectionStateConnected:
		return "Connected"
	case ConnectionStateFailed:
		return "Failed"
	case ConnectionStateDisconnected:
		return "Disconnected"
	default:
		return "Invalid"
	}
}
