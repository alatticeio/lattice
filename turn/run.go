package turn

func Start(cfg *TurnServerConfig) error {
	// Start the TURN server
	turnServer := NewTurnServer(cfg)
	return turnServer.Start()
}
