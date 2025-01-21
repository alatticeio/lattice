package peer

import "linkany/pkg/config"

type EventType int

const (
	// EventType for every service
	AddEvent EventType = iota
	UpdateEvent
	DeleteEvent
)

func (e EventType) String() string {
	switch e {
	case AddEvent:
		return "Add"
	case UpdateEvent:
		return "Update"
	case DeleteEvent:
		return "Delete"
	}
	return "Unknown"
}

type Message struct {
	EventType EventType
	Peer      *config.Peer
}
