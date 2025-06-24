package drp

import (
	"linkany/drp/server"
	"linkany/pkg/log"
)

func Start(listen string) error {
	// Create a new server
	s, err := server.NewServer(&server.ServerConfig{
		Listen: listen,
		Logger: log.NewLogger(log.Loglevel, "drp-signaling"),
	})

	if err != nil {
		return err

	}
	// Start the server
	return s.Start()
}
