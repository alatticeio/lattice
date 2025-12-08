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
	"wireflow/drp"
	"wireflow/internal"
	"wireflow/pkg/log"

	"github.com/wireflowio/ice"
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
	engine       internal.IClient
	offerHandler internal.OfferHandler
	//relayer internal.Relay

	stunUrl         string
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
}

func NewManager(isForceRelay bool, udpMux *ice.UDPMuxDefault,
	universeUdpMux *ice.UniversalUDPMuxDefault,
	engineManager internal.IClient,
	stunUrl string) internal.ProbeManager {
	return &manager{
		agentManager:    drp.NewAgentManager(),
		probers:         make(map[string]internal.Probe),
		isForceRelay:    isForceRelay,
		udpMux:          udpMux,
		universalUdpMux: universeUdpMux,
		stunUrl:         stunUrl,
		engine:          engineManager,
		logger:          log.NewLogger(log.Loglevel, "probe-manager"),
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

		m.logger.Verbosef("gathered candidate: %s", candidate.String())
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

// NewProbe creates a new Probe, is a probe manager
func (m *manager) NewProbe(cfg *internal.ProbeConfig) (internal.Probe, error) {
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
		connectionState: internal.ConnectionStateNew,
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
	case internal.DirectType:
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

func (m *manager) RemoveProbe(key string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.probers, key)
}
