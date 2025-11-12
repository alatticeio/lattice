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

package drp

import (
	"net"
	"sync"
	"wireflow/internal"
	"wireflow/pkg/wferrors"

	"github.com/pion/logging"
	"github.com/pion/randutil"
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

type instance struct {
	lock   sync.Locker
	agents map[string]*internal.Agent
}

func NewAgentManager() internal.AgentManagerFactory {
	return &instance{
		agents: make(map[string]*internal.Agent, 1),
	}
}

func (i *instance) Get(pubKey string) (*internal.Agent, error) {
	if agent, ok := i.agents[pubKey]; ok {
		return agent, nil
	}

	return nil, wferrors.ErrAgentNotFound
}

func (i *instance) Remove(pubKey string) error {
	i.lock.Lock()
	defer i.lock.Unlock()

	if agent, ok := i.agents[pubKey]; ok {
		_ = agent.Close()
		delete(i.agents, pubKey)
		return nil
	}

	return wferrors.ErrAgentNotFound
}

func (i *instance) NewUdpMux(conn net.PacketConn) *ice.UniversalUDPMuxDefault {
	loggerFactory := logging.NewDefaultLoggerFactory()
	loggerFactory.DefaultLogLevel = logging.LogLevelDebug

	universalUdpMux := ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
		Logger:  loggerFactory.NewLogger("wrapper"),
		UDPConn: conn,
		Net:     nil,
	})

	return universalUdpMux
}
