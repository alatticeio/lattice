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
	"time"
	drpgrpc "wireflow/internal/grpc"
	"wireflow/pkg/log"
	"wireflow/pkg/turn"

	"github.com/wireflowio/ice"
)

// ProberManager for managing all Probers
type ProberManager interface {
	NewIceAgent(gatherCh chan interface{}, fn func(state ConnectionState) error) (*ice.Agent, error)
	NewProbe(cfg *ProbeConfig) (Prober, error)
	AddProbe(key string, probe Prober)
	GetProbe(key string) Prober
	RemoveProbe(key string)
}

// Prober 探测接口
type Prober interface {
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

	//GetAgent once agent closed, should recreate a new one
	GetIceAgent() *ice.Agent

	//Restart when disconnected, restart the probe
	Restart() error

	GetCredentials() (string, string, error)

	GetLastCheck() time.Time

	UpdateLastCheck()

	SetConnectType(connType ConnType)
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
	ProberManager           ProberManager
	NodeManager             PeerManager
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

// Checker is the interface for checking the connection.
// DirectChecker and RelayChecker are the two implementations.
type Checker interface {

	// ProbeConnect probes the connection
	ProbeConnect(ctx context.Context, isControlling bool, remoteOffer Offer) error

	// ProbeSuccess will be called when the connection is successful, will add peer to wireguard
	ProbeSuccess(ctx context.Context, addr string) error

	// ProbeFailure will be called when the connection failed, will remove peer from wireguard
	ProbeFailure(ctx context.Context, offer Offer) error
}
