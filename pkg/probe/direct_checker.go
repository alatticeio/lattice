package probe

import (
	"context"
	"github.com/linkanyio/ice"
	"linkany/internal"
	"linkany/internal/direct"
	"linkany/pkg/iface"
	"linkany/pkg/log"
	"net"
	"strings"
	"sync/atomic"
)

const (
	UfragLen = 24
	PwdLen   = 32
)

var (
	Generator_string             = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")
	_                ConnChecker = (*DirectChecker)(nil)
)

// DirectChecker represents present node's connection to remote peer which fetch from control server
type DirectChecker struct {
	logger       *log.Logger
	isStarted    atomic.Bool
	ufrag        string
	pwd          string
	agent        *ice.Agent // agent should gather local candidates before connect, also should add remote candidates
	remoteKey    string
	addr         *net.UDPAddr
	addPeer      func(key string, addr *net.UDPAddr) error
	offerManager internal.OfferManager
	km           *internal.KeyManager
	localKey     uint32
	wgConfiger   iface.WGConfigure
	prober       *Prober
}

type DirectCheckerConfig struct {
	Logger        *log.Logger
	Ufrag         string // local
	Pwd           string
	IsControlling bool
	Agent         *ice.Agent
	Key           string
	WgConfiger    iface.WGConfigure
	LocalKey      uint32
}

func NewDirectChecker(config *DirectCheckerConfig) *DirectChecker {
	if config.Logger == nil {
		config.Logger = log.NewLogger(log.Loglevel, "direct-checker")
	}
	pc := &DirectChecker{
		logger:     config.Logger,
		agent:      config.Agent,
		ufrag:      "",
		pwd:        "",
		remoteKey:  config.Key,
		wgConfiger: config.WgConfiger,
		localKey:   config.LocalKey,
	}

	return pc
}

func (dt *DirectChecker) handleOffer(offer internal.Offer) error {
	o := offer.(*direct.DirectOffer)
	return dt.handleDirectOffer(o)
}

func (dt *DirectChecker) handleDirectOffer(offer *direct.DirectOffer) error {
	// add remote candidate
	candidates := strings.Split(offer.Candidate, ";")
	for _, candString := range candidates {
		if candString == "" {
			continue
		}
		candidate, err := ice.UnmarshalCandidate(candString)

		if err != nil {
			dt.logger.Errorf("unmarshal candidate failed: %v", err)
			continue
		}

		if err = dt.agent.AddRemoteCandidate(candidate); err != nil {
			dt.logger.Errorf("add remote candidate failed: %v", err)
			continue
		}

		dt.logger.Infof("add remote candidate success:%v", candidate.Marshal())
	}

	return nil
}

// ProbeConnect probes the connection
func (dt *DirectChecker) ProbeConnect(ctx context.Context, isControlling bool, remoteOffer internal.Offer) error {
	if dt.isStarted.Load() {
		return nil
	}

	dt.isStarted.Store(true)
	var conn *ice.Conn
	var err error
	candidates, _ := dt.agent.GetRemoteCandidates()

	offer := remoteOffer.(*direct.DirectOffer)

	dt.logger.Infof("remote candidates: %v, current node is controlling: %v", candidates, isControlling)
	if isControlling {
		conn, err = dt.agent.Dial(ctx, offer.Ufrag, offer.Pwd)
	} else {
		conn, err = dt.agent.Accept(ctx, offer.Ufrag, offer.Pwd)
	}

	if err != nil {
		dt.logger.Errorf("peer p2p connection to %s failed: %v", dt.addr.String(), err)
		return dt.OnFailure(remoteOffer) // TODO will set relay checker
	}

	return dt.OnSuccess(conn.RemoteAddr().String())
}

func (dt *DirectChecker) OnSuccess(conn string) error {
	return dt.prober.ProbeSuccess(dt.remoteKey, conn)
}

func (dt *DirectChecker) OnFailure(offer internal.Offer) error {
	return dt.prober.ProbeFailed(dt, offer)
}

func (dt *DirectChecker) Close() error {
	return dt.agent.GracefulClose()
}

func (dt *DirectChecker) SetProber(prober *Prober) {
	dt.prober = prober
}
