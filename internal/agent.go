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

package internal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"wireflow/pkg/log"

	"github.com/pion/logging"
	"github.com/pion/stun/v3"
	"github.com/wireflowio/ice"
)

// AgentManagerFactory is an interface for managing ICE agents.
type AgentManagerFactory interface {
	Get(pubKey string) (*Agent, error)
	Remove(pubKey string) error
	NewUdpMux(conn net.PacketConn) *ice.UniversalUDPMuxDefault
}

//type AgentManager interface {
//	Get(pubKey string) *ice.Agent
//}

// Agent represents an ICE agent with its associated local key.
type Agent struct {
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
func NewAgent(params *AgentConfig) (*Agent, error) {
	var (
		err     error
		agent   *ice.Agent
		stunUri []*stun.URI
		uri     *stun.URI
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
	if agent, err = ice.NewAgent(&ice.AgentConfig{
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

	a := &Agent{
		iceAgent:        agent,
		LocalKey:        ice.NewTieBreaker(),
		universalUdpMux: params.UniversalUdpMux,
		logger:          log.NewLogger(log.Loglevel, "agent"),
	}

	a.started.Store(false)
	return a, nil
}

func (agent *Agent) GetStatus() bool {
	if agent.started.Load() {
		return true
	}
	return false
}

func (agent *Agent) GetUniversalUDPMuxDefault() *ice.UniversalUDPMuxDefault {
	return agent.universalUdpMux
}

func (agent *Agent) OnCandidate(fn func(ice.Candidate)) error {
	return agent.iceAgent.OnCandidate(fn)
}

func (agent *Agent) OnConnectionStateChange(fn func(ice.ConnectionState)) error {
	if fn != nil {
		return agent.iceAgent.OnConnectionStateChange(fn)
	}
	return nil
}

func (agent *Agent) AddRemoteCandidate(candidate ice.Candidate) error {
	if agent.iceAgent == nil {
		return nil
	}
	return agent.iceAgent.AddRemoteCandidate(candidate)
}

func (agent *Agent) GatherCandidates() error {
	if agent.iceAgent == nil {
		return nil
	}
	return agent.iceAgent.GatherCandidates()
}

func (agent *Agent) GetLocalCandidates() ([]ice.Candidate, error) {
	if agent.iceAgent == nil {
		return nil, errors.New("ICE agent is not initialized")
	}
	return agent.iceAgent.GetLocalCandidates()
}

func (agent *Agent) GetTieBreaker() uint64 {
	if agent.iceAgent == nil {
		return 0
	}
	return agent.iceAgent.GetTieBreaker()
}

func (agent *Agent) Dial(ctx context.Context, remoteUfrag, remotePwd string) (*ice.Conn, error) {
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

func (agent *Agent) Accept(ctx context.Context, remoteUfrag, remotePwd string) (*ice.Conn, error) {
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

func (agent *Agent) Close() error {
	if agent.iceAgent == nil {
		return nil
	}
	if err := agent.iceAgent.Close(); err != nil {
		agent.logger.Errorf("failed to close ICE agent: %v", err)
		return err
	}
	return nil
}

func (agent *Agent) GetLocalUserCredentials() (string, string, error) {
	if agent.iceAgent == nil {
		return "", "", errors.New("ICE agent is not initialized")
	}
	return agent.iceAgent.GetLocalUserCredentials()
}

func (agent *Agent) GetRemoteCandidates() ([]ice.Candidate, error) {
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
