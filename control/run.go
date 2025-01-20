package control

import "linkany/control/server"

func Start(listen string) error {
	// Create a new server
	s := server.NewServer(&server.ServerConfig{
		Listen: listen,
	})
	// Start the server
	return s.Start()
}
