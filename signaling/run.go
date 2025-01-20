package signaling

import (
	"linkany/signaling/server"
)

func Start(listen string) error {
	// Create a new server
	s := server.NewServer(&server.ServerConfig{
		Listen: listen,
	})
	// Start the server
	return s.Start()
}
