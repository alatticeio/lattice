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
	"fmt"
	"sync"
	"sync/atomic"
	"wireflow/pkg/log"

	"github.com/pion/logging"
	"github.com/pion/randutil"
	"github.com/pion/stun/v3"
	"github.com/wireflowio/ice"
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

// Agent represents an ICE Agent with its associated local key.
type Agent struct {
	lock            sync.Mutex
	started         atomic.Bool
	logger          *log.Logger
	LocalKey        uint32
	stunUrl         string
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
}

func NewAgent(stunUri string, universalUdpMux *ice.UniversalUDPMuxDefault) *Agent {
	agent := &Agent{
		stunUrl:         stunUri,
		universalUdpMux: universalUdpMux,
		logger:          log.NewLogger(log.Loglevel, "agent"),
	}
	return agent
}

// NewIceAgent create an new ice agent.
func (a *Agent) NewIceAgent(gatherCh chan interface{}) (*ice.Agent, error) {
	var (
		err      error
		iceAgent *ice.Agent
		stunUri  []*stun.URI
		uri      *stun.URI
	)
	l := logging.NewDefaultLoggerFactory()
	l.DefaultLogLevel = logging.LogLevelDebug
	if uri, err = stun.ParseURI(fmt.Sprintf("%s:%s", "stun", a.stunUrl)); err != nil {
		return nil, err
	}

	uri.Username = "admin"
	uri.Password = "admin"
	stunUri = append(stunUri, uri)
	f := logging.NewDefaultLoggerFactory()
	f.DefaultLogLevel = logging.LogLevelDebug
	if iceAgent, err = ice.NewAgent(&ice.AgentConfig{
		NetworkTypes:   []ice.NetworkType{ice.NetworkTypeUDP4},
		UDPMux:         a.universalUdpMux.UDPMuxDefault,
		UDPMuxSrflx:    a.universalUdpMux,
		Tiebreaker:     uint64(ice.NewTieBreaker()),
		Urls:           stunUri,
		LoggerFactory:  f,
		CandidateTypes: []ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypeServerReflexive},
	}); err != nil {
		return nil, err
	}

	iceAgent.OnCandidate(func(candidate ice.Candidate) {
		if candidate == nil {
			a.logger.Verbosef("gathered all candidates")
			close(gatherCh)
			return
		}

		a.logger.Verbosef("gathered candidate: %s", candidate.String())
	})

	return iceAgent, nil
}
