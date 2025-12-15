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

package domain

import (
	"context"
	"net"

	"github.com/wireflowio/ice"
)

// AgentManagerFactory is an interface for managing ICE agents.
type AgentManagerFactory interface {
	Get(pubKey string) (IAgent, error)
	Remove(pubKey string) error
	NewUdpMux(conn net.PacketConn) *ice.UniversalUDPMuxDefault
}

type IAgent interface {
	GetStatus() bool
	GetLocalUserCredentials() (string, string, error)
	GetUniversalUDPMuxDefault() *ice.UniversalUDPMuxDefault
	OnCandidate(fn func(ice.Candidate)) error
	OnConnectionStateChange(fn func(ice.ConnectionState)) error
	AddRemoteCandidate(candidate ice.Candidate) error
	GatherCandidates() error
	GetLocalCandidates() ([]ice.Candidate, error)
	GetRemoteCandidates() ([]ice.Candidate, error)
	Close() error
	Dial(ctx context.Context, remoteUfrag, remotePwd string) (*ice.Conn, error)
	Accept(ctx context.Context, remoteUfrag, remotePwd string) (*ice.Conn, error)
	GetTieBreaker() uint64
}

//type AgentManager interface {
//	Get(pubKey string) *ice.Agent
//}
