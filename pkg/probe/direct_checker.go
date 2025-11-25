// Copyright 2025 Wireflow.io, Inc.
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
	"context"
	"net"
	"strings"
	"sync/atomic"
	"wireflow/internal"
	"wireflow/pkg/log"

	"github.com/wireflowio/ice"
)

const (
	UfragLen = 24
	PwdLen   = 32
)

var (
	Generator_string                  = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")
	_                internal.Checker = (*directChecker)(nil)
)

// directChecker represents present node's connection to remote peer which fetch from control server
type directChecker struct {
	logger       *log.Logger
	isStarted    atomic.Bool
	to           string
	addr         *net.UDPAddr
	addPeer      func(key string, addr *net.UDPAddr) error
	offerManager internal.OfferHandler
	km           internal.KeyManager
	localKey     uint64
	wgConfiger   internal.Configurer
	prober       internal.Probe
}

type DirectCheckerConfig struct {
	Logger        *log.Logger
	Ufrag         string // local
	Pwd           string
	IsControlling bool
	Agent         *internal.Agent
	Key           string
	WgConfiger    internal.Configurer
	LocalKey      uint64
	Prober        internal.Probe
}

func NewDirectChecker(config *DirectCheckerConfig) *directChecker {
	if config.Logger == nil {
		config.Logger = log.NewLogger(log.Loglevel, "direct-checker")
	}
	pc := &directChecker{
		logger: config.Logger,
		//agent:      config.Agent,
		prober:     config.Prober,
		to:         config.Key,
		wgConfiger: config.WgConfiger,
		localKey:   config.LocalKey,
	}

	return pc
}

func (dt *directChecker) HandleOffer(offer internal.Offer) error {
	o := offer.(*internal.DirectOffer)
	if dt.prober == nil {

	}
	return dt.handleDirectOffer(o)
}

func (dt *directChecker) handleDirectOffer(offer *internal.DirectOffer) error {
	// add remote candidate
	candidates := strings.Split(offer.Candidate, ";")
	for _, candString := range candidates {
		if candString == "" {
			continue
		}
		candidate, err := ice.UnmarshalCandidate(candString)

		if err != nil {
			dt.logger.Errorf("unmarshal candidate failed: %v", err)
			continue
		}

		agent := dt.prober.GetProbeAgent()

		if err = agent.AddRemoteCandidate(candidate); err != nil {
			dt.logger.Errorf("add remote candidate failed: %v", err)
			continue
		}

		dt.logger.Infof("add remote candidate success:%v, agent: %v", candidate.Marshal(), agent)
	}

	return nil
}

// ProbeConnect probes the connection
func (dt *directChecker) ProbeConnect(ctx context.Context, isControlling bool, remoteOffer internal.Offer) error {
	logger := dt.logger
	logger.Infof("starting direct checker, isControlling: %v, remoteOffer: %v, to: %v, addr: %v", isControlling, remoteOffer, dt.to, dt.addr)
	var conn *ice.Conn
	var err error

	agent := dt.prober.GetProbeAgent()
	status := agent.GetStatus()
	if status.Load() {
		logger.Infof("agent has started: %v", status)
		return nil
	}

	candidates, _ := agent.GetRemoteCandidates()

	offer := remoteOffer.(*internal.DirectOffer)

	ufrag, pwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		dt.logger.Errorf("get local user credentials failed: %v", err)
		return dt.ProbeFailure(ctx, remoteOffer)
	}
	logger.Infof("local user credentials, ufrag: %v, pwd: %v, candidates: %v, isControlling: %v, remoteOffer: %v, to: %v, addr: %v", ufrag, pwd, candidates, isControlling, remoteOffer, dt.to, dt.addr)
	if isControlling {
		conn, err = agent.Dial(ctx, offer.Ufrag, offer.Pwd)
	} else {
		conn, err = agent.Accept(ctx, offer.Ufrag, offer.Pwd)
	}

	if err != nil {
		dt.logger.Errorf("peer p2p connection to %s failed: %v", dt.addr.String(), err)
		return dt.ProbeFailure(ctx, remoteOffer)
	}

	return dt.ProbeSuccess(ctx, conn.RemoteAddr().String())
}

func (dt *directChecker) ProbeSuccess(ctx context.Context, conn string) error {
	return dt.prober.ProbeSuccess(ctx, dt.to, conn)
}

func (dt *directChecker) ProbeFailure(ctx context.Context, offer internal.Offer) error {
	return dt.prober.ProbeFailed(ctx, dt, offer)
}

func (dt *directChecker) Close() error {
	return dt.prober.GetProbeAgent().Close()
}

func (dt *directChecker) SetProbe(prober internal.Probe) {
	dt.prober = prober
}
