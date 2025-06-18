package internal

import (
	"context"
	drpgrpc "linkany/drp/grpc"
	"linkany/pkg/config"
	"linkany/pkg/log"
	"linkany/turn/client"
	"time"
)

type Probe interface {
	// Start the check process
	Start(srcKey, dstKey string) error

	SendOffer(frameType drpgrpc.MessageType, srcKey, dstKey string) error

	HandleOffer(offer Offer) error

	ProbeConnect(ctx context.Context, offer Offer) error

	ProbeSuccess(publicKey string, conn string) error

	ProbeFailed(checker Checker, offer Offer) error

	IsForceRelay() bool

	GetConnState() ConnectionState

	UpdateConnectionState(state ConnectionState)

	OnConnectionStateChange(state ConnectionState) error

	ProbeDone() chan interface{}

	//GetProbeAgent once agent closed, should recreate a new one
	GetProbeAgent() *Agent

	//Restart when disconnected, restart the probe
	Restart() error

	GetGatherChan() chan interface{}

	TieBreaker() uint64

	GetCredentials() (string, string, error)

	GetLastCheck() time.Time

	UpdateLastCheck()
}

type ProbeManager interface {
	NewAgent(gatherCh chan interface{}, fn func(state ConnectionState) error) (*Agent, error)
	NewProbe(cfg *ProberConfig) (Probe, error)
	AddProbe(key string, probe Probe)
	GetProbe(key string) Probe
	Remove(key string)
}

type ProberConfig struct {
	Logger                  *log.Logger
	StunUri                 string
	IsControlling           bool
	IsForceRelay            bool
	ConnType                ConnType
	DirectChecker           Checker
	RelayChecker            Checker
	LocalKey                uint32
	OfferManager            OfferHandler
	WGConfiger              ConfigureManager
	ProberManager           ProbeManager
	NodeManager             *config.NodeManager
	From                    string
	To                      string
	TurnClient              *client.Client
	Relayer                 Relay
	SignalingChannel        chan *drpgrpc.DrpMessage
	Ufrag                   string
	Pwd                     string
	GatherChan              chan interface{}
	OnConnectionStateChange func(state ConnectionState) error
}
