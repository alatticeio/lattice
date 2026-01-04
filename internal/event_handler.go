package internal

type EventHandler interface {
	HandleEvent() error
}
