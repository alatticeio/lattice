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

package manager

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"wireflow/internal/core/domain"
	"wireflow/pkg/log"
	"wireflow/pkg/wferrors"

	"github.com/pion/logging"
	"github.com/pion/randutil"
	"github.com/pion/stun/v3"
	"github.com/wireflowio/ice"
)

var (
	_ domain.IAgent              = (*agent)(nil)
	_ domain.AgentManagerFactory = (*agentFactory)(nil)
)

const (
	runesAlpha                 = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	runesDigit                 = "0123456789"
	runesCandidateIDFoundation = runesAlpha + runesDigit + "+/"

	lenUFrag = 16
	lenPwd   = 32
)

var (
	globalMathRandomGenerator = randutil.NewMathRandomGenerator()
)

type agentFactory struct {
	lock   sync.Locker
	agents map[string]domain.IAgent
}

func NewAgentManagerFactory() domain.AgentManagerFactory {
	return &agentFactory{
		agents: make(map[string]domain.IAgent, 1),
	}
}

func (i *agentFactory) Get(pubKey string) (domain.IAgent, error) {
	if agent, ok := i.agents[pubKey]; ok {
		return agent, nil
	}

	return nil, wferrors.ErrAgentNotFound
}

func (i *agentFactory) Remove(pubKey string) error {
	i.lock.Lock()
	defer i.lock.Unlock()

	if agent, ok := i.agents[pubKey]; ok {
		_ = agent.Close()
		delete(i.agents, pubKey)
		return nil
	}

	return wferrors.ErrAgentNotFound
}

func (i *agentFactory) NewUdpMux(conn net.PacketConn) *ice.UniversalUDPMuxDefault {
	loggerFactory := logging.NewDefaultLoggerFactory()
	loggerFactory.DefaultLogLevel = logging.LogLevelDebug

	universalUdpMux := ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
		Logger:  loggerFactory.NewLogger("wrapper"),
		UDPConn: conn,
		Net:     nil,
	})

	return universalUdpMux
}

// agent represents an ICE agent with its associated local key.
type agent struct {
	lock            sync.Mutex
	started         atomic.Bool
	logger          *log.Logger
	iceAgent        *ice.Agent
	LocalKey        uint32
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
}

// NewAgent creates a new ICE agent, will use to gather candidates
// each peer will create an agent for connection establishment
func NewAgent(params *AgentConfig) (*agent, error) {
	var (
		err      error
		iceAgent *ice.Agent
		stunUri  []*stun.URI
		uri      *stun.URI
	)

	l := logging.NewDefaultLoggerFactory()
	l.DefaultLogLevel = logging.LogLevelDebug
	if uri, err = stun.ParseURI(fmt.Sprintf("%s:%s", "stun", params.StunUrl)); err != nil {
		return nil, err
	}

	uri.Username = "admin"
	uri.Password = "admin"
	stunUri = append(stunUri, uri)
	f := logging.NewDefaultLoggerFactory()
	f.DefaultLogLevel = logging.LogLevelDebug
	if iceAgent, err = ice.NewAgent(&ice.AgentConfig{
		NetworkTypes:   []ice.NetworkType{ice.NetworkTypeUDP4},
		UDPMux:         params.UniversalUdpMux.UDPMuxDefault,
		UDPMuxSrflx:    params.UniversalUdpMux,
		Tiebreaker:     uint64(ice.NewTieBreaker()),
		Urls:           stunUri,
		LoggerFactory:  f,
		CandidateTypes: []ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypeServerReflexive},
	}); err != nil {
		return nil, err
	}

	a := &agent{
		iceAgent:        iceAgent,
		LocalKey:        ice.NewTieBreaker(),
		universalUdpMux: params.UniversalUdpMux,
		logger:          log.NewLogger(log.Loglevel, "agent"),
	}

	a.started.Store(false)
	return a, nil
}

func (agent *agent) GetStatus() bool {
	if agent.started.Load() {
		return true
	}
	return false
}

func (agent *agent) GetUniversalUDPMuxDefault() *ice.UniversalUDPMuxDefault {
	return agent.universalUdpMux
}

func (agent *agent) OnCandidate(fn func(ice.Candidate)) error {
	return agent.iceAgent.OnCandidate(fn)
}

func (agent *agent) OnConnectionStateChange(fn func(ice.ConnectionState)) error {
	if fn != nil {
		return agent.iceAgent.OnConnectionStateChange(fn)
	}
	return nil
}

func (agent *agent) AddRemoteCandidate(candidate ice.Candidate) error {
	if agent.iceAgent == nil {
		return nil
	}
	return agent.iceAgent.AddRemoteCandidate(candidate)
}

func (agent *agent) GatherCandidates() error {
	if agent.iceAgent == nil {
		return nil
	}
	return agent.iceAgent.GatherCandidates()
}

func (agent *agent) GetLocalCandidates() ([]ice.Candidate, error) {
	if agent.iceAgent == nil {
		return nil, errors.New("ICE agent is not initialized")
	}
	return agent.iceAgent.GetLocalCandidates()
}

func (agent *agent) GetTieBreaker() uint64 {
	if agent.iceAgent == nil {
		return 0
	}
	return agent.iceAgent.GetTieBreaker()
}

func (agent *agent) Dial(ctx context.Context, remoteUfrag, remotePwd string) (*ice.Conn, error) {
	if agent.iceAgent == nil {
		return nil, errors.New("ICE agent is not initialized")
	}

	conn, err := agent.iceAgent.Dial(ctx, remoteUfrag, remotePwd)
	if err != nil {
		agent.logger.Errorf("failed to accept ICE connection: %v", err)
		return nil, err
	}
	return conn, nil
}

func (agent *agent) Accept(ctx context.Context, remoteUfrag, remotePwd string) (*ice.Conn, error) {
	if agent.iceAgent == nil {
		return nil, errors.New("ICE agent is not initialized")
	}

	conn, err := agent.iceAgent.Accept(ctx, remoteUfrag, remotePwd)
	if err != nil {
		agent.logger.Errorf("failed to accept ICE connection: %v", err)
		return nil, err
	}
	return conn, nil
}

func (agent *agent) Close() error {
	if agent.iceAgent == nil {
		return nil
	}
	if err := agent.iceAgent.Close(); err != nil {
		agent.logger.Errorf("failed to close ICE agent: %v", err)
		return err
	}
	return nil
}

func (agent *agent) GetLocalUserCredentials() (string, string, error) {
	if agent.iceAgent == nil {
		return "", "", errors.New("ICE agent is not initialized")
	}
	return agent.iceAgent.GetLocalUserCredentials()
}

func (agent *agent) GetRemoteCandidates() ([]ice.Candidate, error) {
	if agent.iceAgent == nil {
		return nil, errors.New("ICE agent is not initialized")
	}
	return agent.iceAgent.GetRemoteCandidates()
}

// AgentConfig holds the configuration for creating a new ICE agent.
type AgentConfig struct {
	StunUrl         string
	UniversalUdpMux *ice.UniversalUDPMuxDefault
}
