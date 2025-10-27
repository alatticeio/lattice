package internal

import (
	"context"
	"time"
	drpgrpc "wireflow/drp/grpc"
	"wireflow/pkg/log"
	"wireflow/turn/client"
)

type Probe interface {
	// Start the check process
	Start(ctx context.Context, srcKey, dstKey string) error

	SendOffer(ctx context.Context, frameType drpgrpc.MessageType, srcKey, dstKey string) error

	HandleOffer(ctx context.Context, offer Offer) error

	ProbeConnect(ctx context.Context, offer Offer) error

	ProbeSuccess(ctx context.Context, publicKey string, conn string) error

	ProbeFailed(ctx context.Context, checker Checker, offer Offer) error

	GetConnState() ConnectionState

	UpdateConnectionState(state ConnectionState)

	OnConnectionStateChange(state ConnectionState) error

	ProbeDone() chan interface{}

	//GetProbeAgent once agent closed, should recreate a new one
	GetProbeAgent() *Agent

	//Restart when disconnected, restart the probe
	Restart() error

	TieBreaker() uint64

	GetCredentials() (string, string, error)

	GetLastCheck() time.Time

	UpdateLastCheck()

	SetConnectType(connType ConnectType)
}

type ProbeManager interface {
	NewAgent(gatherCh chan interface{}, fn func(state ConnectionState) error) (*Agent, error)
	NewProbe(cfg *ProbeConfig) (Probe, error)
	AddProbe(key string, probe Probe)
	GetProbe(key string) Probe
	RemoveProbe(key string)
}

type ProbeConfig struct {
	Logger                  *log.Logger
	StunUri                 string
	IsControlling           bool
	IsForceRelay            bool
	ConnType                ConnectType
	DirectChecker           Checker
	RelayChecker            Checker
	LocalKey                uint32
	WGConfiger              ConfigureManager
	OfferHandler            OfferHandler
	ProberManager           ProbeManager
	NodeManager             *NodeManager
	From                    string
	To                      string
	TurnManager             *client.TurnManager
	SignalingChannel        chan *drpgrpc.DrpMessage
	Ufrag                   string
	Pwd                     string
	GatherChan              chan interface{}
	OnConnectionStateChange func(state ConnectionState) error

	ConnectType ConnectType
}
