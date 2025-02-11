package server

// Handler a handler will created when two client exchange data with each other
type Handler struct {
	channel chan *ForwardMessage
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Handle() {

}
