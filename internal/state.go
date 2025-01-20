package internal

type ConnectionState int

const (
	ConnectionStateNew ConnectionState = iota
	ConnectionStateChecking
	ConnectionStateFailed
	ConnectionStateConnected
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
	default:
		return "Invalid"
	}
}
