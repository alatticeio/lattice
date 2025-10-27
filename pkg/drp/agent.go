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
