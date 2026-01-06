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

package transport

import (
	"context"
	"fmt"
	"sync/atomic"
	"wireflow/internal/grpc"
	"wireflow/internal/log"

	"github.com/pion/logging"
	"github.com/wireflowio/ice"
	"google.golang.org/protobuf/proto"
)

type AgentWrapper struct {
	sender  func(ctx context.Context, peerId string, data []byte) error
	localId string
	peerId  string
	log     *log.Logger
	*ice.Agent
	IsCredentialsInited atomic.Bool
	RUfrag              string
	RPwd                string
	RTieBreaker         uint64
}

type AgentConfig struct {
	Send    func(ctx context.Context, peerId string, data []byte) error
	LocalId string
	PeerID  string
	StunURI string
	//连接成功时回调
	onCall func(peerId string, address string) error
}

func NewAgent(cfg *AgentConfig) (*AgentWrapper, error) {
	f := logging.NewDefaultLoggerFactory()
	f.DefaultLogLevel = logging.LogLevelDebug
	// 创建新 Agent
	iceAgent, err := ice.NewAgent(&ice.AgentConfig{
		// 建议：对于每个 Peer，使用独立的随机凭证
		NetworkTypes:  []ice.NetworkType{ice.NetworkTypeUDP4},
		Urls:          []*ice.URL{{Scheme: ice.SchemeTypeSTUN, Host: "81.68.109.143", Port: 3478}},
		Tiebreaker:    uint64(ice.NewTieBreaker()),
		LoggerFactory: f,
	})

	var agent *AgentWrapper
	if err == nil {
		agent = &AgentWrapper{
			Agent:   iceAgent,
			log:     log.NewLogger(log.Loglevel, "agent-wrapper"),
			sender:  cfg.Send,
			localId: cfg.LocalId,
			peerId:  cfg.PeerID,
		}
		// 绑定状态监听，成功后更新 WireGuard
		agent.OnConnectionStateChange(func(s ice.ConnectionState) {
			if s == ice.ConnectionStateConnected {
				pair, err := agent.GetSelectedCandidatePair()
				if err != nil {
					agent.log.Errorf("Get selected candidate pair error: %v", err)
					return
				}
				cfg.onCall(cfg.PeerID, fmt.Sprintf("%s:%d", pair.Remote.Address(), pair.Remote.Port()))
			}

		})
	}

	if err = agent.OnCandidate(func(candidate ice.Candidate) {
		if candidate == nil {
			return
		}

		if err = agent.sendCandidate(context.Background(), candidate); err != nil {
			agent.log.Errorf("Send candidate error: %v", err)
		}

	}); err != nil {
		return nil, err
	}

	return agent, err
}

func (agent *AgentWrapper) sendCandidate(ctx context.Context, candidate ice.Candidate) error {
	ufrag, pwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		return err
	}
	packet := &grpc.SignalPacket{
		Type:     grpc.PacketType_OFFER,
		SenderId: agent.localId,
		Payload: &grpc.SignalPacket_Offer{
			Offer: &grpc.Offer{
				Ufrag:      ufrag,
				Pwd:        pwd,
				TieBreaker: agent.GetTieBreaker(),
				Candidate:  candidate.Marshal(),
			},
		},
	}

	data, err := proto.Marshal(packet)
	if err != nil {
		agent.log.Errorf("Marshal packet error: %v", err)
		return err
	}

	return agent.sender(ctx, agent.peerId, data)
}
