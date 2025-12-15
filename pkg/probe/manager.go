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
	_ domain.ProbeManager = (*probeInstance)(nil)
)

type probeInstance struct {
	logger       *log.Logger
	lock         sync.Mutex
	probers      map[string]domain.Probe
	wgLock       sync.Mutex
	isForceRelay bool
	agentManager domain.AgentManagerFactory
	engine       domain.IClient
	offerHandler domain.OfferHandler

	stunUrl         string
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
}

func NewManager(isForceRelay bool, udpMux *ice.UDPMuxDefault,
	universeUdpMux *ice.UniversalUDPMuxDefault,
	engineManager domain.IClient,
	stunUrl string) domain.ProbeManager {
	return &probeInstance{
		agentManager:    manager.NewAgentManagerFactory(),
		probers:         make(map[string]domain.Probe),
		isForceRelay:    isForceRelay,
		udpMux:          udpMux,
		universalUdpMux: universeUdpMux,
		stunUrl:         stunUrl,
		engine:          engineManager,
		logger:          log.NewLogger(log.Loglevel, "probe-manager"),
	}
}

func (m *probeInstance) NewAgent(gatherCh chan interface{}, fn func(state domain.ConnectionState) error) (domain.IAgent, error) {
	var (
		err   error
		agent domain.IAgent
	)
	if agent, err = manager.NewAgent(&manager.AgentConfig{
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

		m.logger.Verbosef("gathered candidate: %s", candidate.String())
	}); err != nil {
		return nil, err
	}

	if err = agent.OnConnectionStateChange(func(state ice.ConnectionState) {
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

	return agent, nil
}

// NewProbe creates a new Probe, is a probe manager
func (m *probeInstance) NewProbe(cfg *domain.ProbeConfig) (domain.Probe, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	p := m.probers[cfg.To] // check if probe already exists
	if p != nil {
		return p, nil
	}

	var (
		err error
	)

	newProbe := &probe{
		logger:          log.NewLogger(log.Loglevel, "probe"),
		connectionState: domain.ConnectionStateNew,
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
		if newProbe.agent, err = m.NewAgent(newProbe.gatherCh, newProbe.OnConnectionStateChange); err != nil {
			return nil, err
		}

		if err = newProbe.agent.GatherCandidates(); err != nil {
			return nil, err
		}
	}

	m.probers[cfg.To] = newProbe

	return newProbe, nil
}

func (m *probeInstance) AddProbe(key string, prober domain.Probe) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.probers[key] = prober
}

func (m *probeInstance) GetProbe(key string) domain.Probe {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.probers[key]
}

func (m *probeInstance) RemoveProbe(key string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.probers, key)
}
