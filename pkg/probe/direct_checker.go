package probe

import (
	"context"
	"github.com/linkanyio/ice"
	"linkany/internal"
	"linkany/internal/direct"
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
	Generator_string                  = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")
	_                internal.Checker = (*directChecker)(nil)
)

// directChecker represents present node's connection to remote peer which fetch from control server
type directChecker struct {
	logger       *log.Logger
	isStarted    atomic.Bool
	to           string
	addr         *net.UDPAddr
	addPeer      func(key string, addr *net.UDPAddr) error
	offerManager internal.OfferHandler
	km           internal.KeyManager
	localKey     uint64
	wgConfiger   internal.ConfigureManager
	prober       internal.Probe
}

type DirectCheckerConfig struct {
	Logger        *log.Logger
	Ufrag         string // local
	Pwd           string
	IsControlling bool
	Agent         *internal.Agent
	Key           string
	WgConfiger    internal.ConfigureManager
	LocalKey      uint64
	Prober        internal.Probe
}

func NewDirectChecker(config *DirectCheckerConfig) *directChecker {
	if config.Logger == nil {
		config.Logger = log.NewLogger(log.Loglevel, "direct-checker")
	}
	pc := &directChecker{
		logger: config.Logger,
		//agent:      config.Agent,
		prober:     config.Prober,
		to:         config.Key,
		wgConfiger: config.WgConfiger,
		localKey:   config.LocalKey,
	}

	return pc
}

func (dt *directChecker) HandleOffer(offer internal.Offer) error {
	o := offer.(*direct.DirectOffer)
	if dt.prober == nil {

	}
	return dt.handleDirectOffer(o)
}

func (dt *directChecker) handleDirectOffer(offer *direct.DirectOffer) error {
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

		agent := dt.prober.GetProbeAgent()

		if err = agent.AddRemoteCandidate(candidate); err != nil {
			dt.logger.Errorf("add remote candidate failed: %v", err)
			continue
		}

		dt.logger.Infof("add remote candidate success:%v, agent: %v", candidate.Marshal(), agent)
	}

	return nil
}

// ProbeConnect probes the connection
func (dt *directChecker) ProbeConnect(ctx context.Context, isControlling bool, remoteOffer internal.Offer) error {
	var conn *ice.Conn
	var err error

	agent := dt.prober.GetProbeAgent()
	candidates, _ := agent.GetRemoteCandidates()

	getCh := make(chan interface{})
	if candidates == nil {
		go func() {
			for {
				candidates, err = agent.GetRemoteCandidates()
				if err != nil {
					dt.logger.Errorf("get remote candidates failed: %v", err)
					continue
				}
				if len(candidates) > 0 {
					dt.logger.Infof("remote candidates: %v", candidates)
					close(getCh)
					return
				}
			}
		}()
	}

	select {
	case <-ctx.Done():
		dt.logger.Errorf("probe connect context done: %v", ctx.Err())
		return dt.ProbeFailure(remoteOffer)
	case <-getCh:
		offer := remoteOffer.(*direct.DirectOffer)

		ufrag, pwd, err := agent.GetLocalUserCredentials()
		if err != nil {
			dt.logger.Errorf("get local user credentials failed: %v", err)
			return dt.ProbeFailure(remoteOffer)
		}
		dt.logger.Infof("===========agent %v, remote candidates: %v, current node is controlling: %v, local ufrag: %v, pwd: %v, remote ufrag: %v, pwd: %v", agent, candidates, isControlling, ufrag, pwd, offer.Ufrag, offer.Pwd)
		if isControlling {
			conn, err = agent.Dial(ctx, offer.Ufrag, offer.Pwd)
		} else {
			conn, err = agent.Accept(ctx, offer.Ufrag, offer.Pwd)
		}

		if err != nil {
			dt.logger.Errorf("peer p2p connection to %s failed: %v", dt.addr.String(), err)
			return dt.ProbeFailure(remoteOffer) // TODO will set relay checker
		}

		return dt.ProbeSuccess(conn.RemoteAddr().String())
	}
}

func (dt *directChecker) ProbeSuccess(conn string) error {
	return dt.prober.ProbeSuccess(dt.to, conn)
}

func (dt *directChecker) ProbeFailure(offer internal.Offer) error {
	return dt.prober.ProbeFailed(dt, offer)
}

func (dt *directChecker) Close() error {
	return dt.prober.GetProbeAgent().Close()
}

func (dt *directChecker) SetProbe(prober internal.Probe) {
	dt.prober = prober
}
