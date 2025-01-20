package turn

import "linkany/turn/server"

func Start(cfg *server.TurnServerConfig) error {
	// Start the TURN server
	turnServer := server.NewTurnServer(cfg)
	return turnServer.Start()
}
