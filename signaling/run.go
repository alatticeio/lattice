package signaling

import (
	"linkany/signaling/server"
)

func Start(listen string) error {
	// Create a new server
	s, err := server.NewServer(&server.ServerConfig{
		Listen: listen,
	})

	if err != nil {
		return err

	}
	// Start the server
	return s.Start()
}
