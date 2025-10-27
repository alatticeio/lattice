package http

type Action string

const (
	Register     Action = "register"
	Unregister          = "unregister"
	JoinNetwork         = "join_network"
	LeaveNetwork        = "leave_network"
)

func (a Action) String() string {
	return string(a)
}

func IsValidAction(action string) bool {
	switch Action(action) {
	case Register, Unregister, JoinNetwork, LeaveNetwork:
		return true
	default:
		return false
	}
}

func ActionFromString(action string) Action {
	switch Action(action) {
	case Register:
		return Register
	case Unregister:
		return Unregister
	case JoinNetwork:
		return JoinNetwork
	case LeaveNetwork:
		return LeaveNetwork
	default:
		return ""
	}
}
