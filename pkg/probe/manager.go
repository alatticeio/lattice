package probe

import (
	"github.com/linkanyio/ice"
	"linkany/internal"
	"linkany/pkg/drp"
	"linkany/pkg/log"
	"sync"
)

var (
	_ internal.ProbeManager = (*manager)(nil)
)

type manager struct {
	logger       *log.Logger
	lock         sync.Mutex
	probers      map[string]internal.Probe
	wgLock       sync.Mutex
	isForceRelay bool
	agentManager internal.AgentManagerFactory
	engine       internal.EngineManager
	//relayer internal.Relay

	stunUrl         string
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
}

func NewManager(isForceRelay bool, udpMux *ice.UDPMuxDefault,
	universeUdpMux *ice.UniversalUDPMuxDefault,
	relayer internal.Relay,
	engineManager internal.EngineManager,
	stunUrl string) internal.ProbeManager {
	return &manager{
		agentManager:    drp.NewAgentManager(),
		probers:         make(map[string]internal.Probe),
		isForceRelay:    isForceRelay,
		udpMux:          udpMux,
		universalUdpMux: universeUdpMux,
		stunUrl:         stunUrl,
		engine:          engineManager,
		logger:          log.NewLogger(log.Loglevel, "probe-manager "),
	}
}

func (m *manager) NewAgent(gatherCh chan interface{}, fn func(state internal.ConnectionState) error) (*internal.Agent, error) {
	var (
		err   error
		agent *internal.Agent
	)
	if agent, err = internal.NewAgent(&internal.AgentConfig{
		StunUrl:         m.stunUrl,
		UniversalUdpMux: m.universalUdpMux,
	}); err != nil {
		return nil, err
	}

	if err = agent.OnCandidate(func(candidate ice.Candidate) {
		if candidate == nil {
			m.logger.Verbosef("gathered all candidates")
			close(gatherCh)
			return
		}

		m.logger.Verbosef("gathered candidate: %s for %s", candidate.String())
	}); err != nil {
		return nil, err
	}

	if err = agent.OnConnectionStateChange(func(state ice.ConnectionState) {
		switch state {
		case ice.ConnectionStateFailed:
			fn(internal.ConnectionStateFailed)
		case ice.ConnectionStateConnected:
			fn(internal.ConnectionStateConnected)
		case ice.ConnectionStateChecking:
			fn(internal.ConnectionStateChecking)
		case ice.ConnectionStateDisconnected:
			fn(internal.ConnectionStateDisconnected)
		case ice.ConnectionStateNew:
			fn(internal.ConnectionStateNew)
		}
	}); err != nil {
		return nil, err
	}

	return agent, nil
}

// NewProbe creates a new Probe, is a prober manager
func (m *manager) NewProbe(cfg *internal.ProberConfig) (internal.Probe, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	probe := m.probers[cfg.To] // check if probe already exists
	if probe != nil {
		return probe, nil
	}

	var (
		err error
	)

	p := &prober{
		logger:          log.NewLogger(log.Loglevel, "probe "),
		connectionState: internal.ConnectionStateNew,
		gatherCh:        cfg.GatherChan,
		directChecker:   cfg.DirectChecker,
		relayChecker:    cfg.RelayChecker,
		offerHandler:    cfg.OfferManager,
		wgConfiger:      m.engine.GetWgConfiger(),
		proberManager:   cfg.ProberManager,
		nodeManager:     cfg.NodeManager,
		isForceRelay:    cfg.IsForceRelay,
		turnClient:      cfg.TurnClient,
		from:            cfg.From,
		to:              cfg.To,
		done:            make(chan interface{}),
	}

	switch p.connectType {
	case internal.DirectType:
		if p.agent, err = m.NewAgent(p.gatherCh, p.OnConnectionStateChange); err != nil {
			return nil, err
		}

		if err = p.agent.GatherCandidates(); err != nil {
			return nil, err
		}
	}

	m.probers[cfg.To] = p

	return p, nil
}

func (m *manager) AddProbe(key string, prober internal.Probe) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.probers[key] = prober
}

func (m *manager) GetProbe(key string) internal.Probe {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.probers[key]
}

func (m *manager) Remove(key string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.probers, key)
}
