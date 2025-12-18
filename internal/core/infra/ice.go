package infra

import (
	"net"

	"github.com/pion/logging"
	"github.com/wireflowio/ice"
)

func NewUdpMux(conn net.PacketConn) *ice.UniversalUDPMuxDefault {
	loggerFactory := logging.NewDefaultLoggerFactory()
	loggerFactory.DefaultLogLevel = logging.LogLevelDebug

	universalUdpMux := ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
		Logger:  loggerFactory.NewLogger("wrapper"),
		UDPConn: conn,
		Net:     nil,
	})

	return universalUdpMux
}
