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
	"time"
	drpgrpc "wireflow/internal/grpc"
	"wireflow/pkg/log"
	"wireflow/pkg/turn"
)

type Probe interface {
	// Start the check process
	Start(ctx context.Context, srcKey, dstKey string) error

	SendOffer(ctx context.Context, frameType drpgrpc.MessageType, srcKey, dstKey string) error

	HandleOffer(ctx context.Context, offer Offer) error

	ProbeConnect(ctx context.Context, offer Offer) error

	ProbeSuccess(ctx context.Context, publicKey string, conn string) error

	ProbeFailed(ctx context.Context, checker Checker, offer Offer) error

	GetConnState() ConnectionState

	UpdateConnectionState(state ConnectionState)

	OnConnectionStateChange(state ConnectionState) error

	ProbeDone() chan interface{}

	//GetProbeAgent once agent closed, should recreate a new one
	GetProbeAgent() *Agent

	//Restart when disconnected, restart the probe
	Restart() error

	TieBreaker() uint64

	GetCredentials() (string, string, error)

	GetLastCheck() time.Time

	UpdateLastCheck()

	SetConnectType(connType ConnType)
}

type ProbeManager interface {
	NewAgent(gatherCh chan interface{}, fn func(state ConnectionState) error) (*Agent, error)
	NewProbe(cfg *ProbeConfig) (Probe, error)
	AddProbe(key string, probe Probe)
	GetProbe(key string) Probe
	RemoveProbe(key string)
}

type ProbeConfig struct {
	Logger                  *log.Logger
	StunUri                 string
	IsControlling           bool
	IsForceRelay            bool
	ConnType                ConnType
	DirectChecker           Checker
	RelayChecker            Checker
	LocalKey                uint32
	WGConfiger              Configurer
	OfferHandler            OfferHandler
	ProberManager           ProbeManager
	NodeManager             *PeerManager
	From                    string
	To                      string
	TurnManager             *turn.TurnManager
	SignalingChannel        chan *drpgrpc.DrpMessage
	Ufrag                   string
	Pwd                     string
	GatherChan              chan interface{}
	OnConnectionStateChange func(state ConnectionState) error

	ConnectType ConnType
}
