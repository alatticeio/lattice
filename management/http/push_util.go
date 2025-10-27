package http

// Callback will be called when a client register or disconnect, and other event happened
func (s *Server) Callback(ctx *CallbackContext) error {
	switch ctx.Action {
	case Register:
		// handle register action
	}
	return nil
}
