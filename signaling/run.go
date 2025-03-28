package signaling

import (
	"linkany/pkg/log"
	"linkany/signaling/server"
)

func Start(listen string) error {
	// Create a new server
	s, err := server.NewServer(&server.ServerConfig{
		Listen: listen,
		Logger: log.NewLogger(log.Loglevel, "signaling"),
	})

	if err != nil {
		return err

	}
	// Start the server
	return s.Start()
}
