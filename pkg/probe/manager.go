// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package probe

import (
	"sync"
	"wireflow/internal/core/domain"
	"wireflow/internal/core/manager"
	"wireflow/pkg/log"

	"github.com/wireflowio/ice"
)

var (
	_ domain.ProberManager = (*proberManager)(nil)
)

type proberManager struct {
	logger          *log.Logger
	lock            sync.Mutex
	probers         map[string]domain.Prober
	agent           *manager.Agent
	wgLock          sync.Mutex
	isForceRelay    bool
	engine          domain.Client
	offerHandler    domain.OfferHandler
	stunUrl         string
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
}

func NewProberManager(isForceRelay bool,
	engineManager domain.Client,
	agent *manager.Agent,
) domain.ProberManager {
	return &proberManager{
		probers:      make(map[string]domain.Prober),
		isForceRelay: isForceRelay,
		agent:        agent,
		engine:       engineManager,
		logger:       log.NewLogger(log.Loglevel, "probe-manager"),
	}
}

func (m *proberManager) NewIceAgent(gatherCh chan interface{}, fn func(state domain.ConnectionState) error) (*ice.Agent, error) {
	var (
		err      error
		iceAgent *ice.Agent
	)

	iceAgent, err = m.agent.NewIceAgent()
	if err != nil {
		return nil, err
	}
	if err = iceAgent.OnCandidate(func(candidate ice.Candidate) {
		if candidate == nil {
			m.logger.Verbosef("gathered all candidates")
			close(gatherCh)
			return
		}

		m.logger.Verbosef("gathered candidate: %s", candidate.String())
	}); err != nil {
		return nil, err
	}

	if err = iceAgent.OnConnectionStateChange(func(state ice.ConnectionState) {
		switch state {
		case ice.ConnectionStateFailed:
			fn(domain.ConnectionStateFailed)
		case ice.ConnectionStateConnected:
			fn(domain.ConnectionStateConnected)
		case ice.ConnectionStateChecking:
			fn(domain.ConnectionStateChecking)
		case ice.ConnectionStateDisconnected:
			fn(domain.ConnectionStateDisconnected)
		case ice.ConnectionStateNew:
			fn(domain.ConnectionStateNew)
		}
	}); err != nil {
		return nil, err
	}

	return iceAgent, nil
}

// NewProbe creates a new Prober, is a probe manager
func (m *proberManager) NewProbe(cfg *domain.ProbeConfig) (domain.Prober, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	p := m.probers[cfg.To] // check if probe already exists
	if p != nil {
		return p, nil
	}

	var (
		err error
	)

	iceAgent, err := m.agent.NewIceAgent()
	if err != nil {
		return nil, err
	}

	newProbe := &probe{
		logger:          log.NewLogger(log.Loglevel, "probe"),
		connectionState: domain.ConnectionStateNew,
		agent:           m.agent,
		iceAgent:        iceAgent,
		gatherCh:        cfg.GatherChan,
		directChecker:   cfg.DirectChecker,
		relayChecker:    cfg.RelayChecker,
		wgConfiger:      m.engine.GetDeviceConfiger(),
		nodeManager:     cfg.NodeManager,
		offerHandler:    cfg.OfferHandler,
		isForceRelay:    cfg.IsForceRelay,
		turnManager:     cfg.TurnManager,
		from:            cfg.From,
		to:              cfg.To,
		done:            make(chan interface{}),
		connectType:     cfg.ConnectType,
		probeManager:    m,
	}

	switch newProbe.connectType {
	case domain.DirectType:
		if err = newProbe.iceAgent.GatherCandidates(); err != nil {
			return nil, err
		}
	}

	m.probers[cfg.To] = newProbe

	return newProbe, nil
}

func (m *proberManager) AddProbe(key string, prober domain.Prober) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.probers[key] = prober
}

func (m *proberManager) GetProbe(key string) domain.Prober {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.probers[key]
}

func (m *proberManager) RemoveProbe(key string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.probers, key)
}
