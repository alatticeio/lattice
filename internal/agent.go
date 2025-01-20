package internal

import (
	"github.com/linkanyio/ice"
	"github.com/pion/logging"
	"github.com/pion/randutil"
	"github.com/pion/stun/v3"
	"net"
	"sync"
	"time"
)

type AgentManager struct {
	lock     sync.Mutex
	agents   map[string]*ice.Agent
	localKey uint32
}

func NewAgentManager() *AgentManager {
	return &AgentManager{
		agents:   make(map[string]*ice.Agent),
		localKey: ice.NewTieBreaker(),
	}
}

func (m *AgentManager) Add(key string, agent *ice.Agent) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.agents[key] = agent
}

func (m *AgentManager) Get(key string) (*ice.Agent, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	agent, ok := m.agents[key]
	return agent, ok
}

func (m *AgentManager) Remove(key string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	agent := m.agents[key]
	agent.Close()
	delete(m.agents, key)
}

func (m *AgentManager) GetLocalKey() uint32 {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.localKey
}

func NewUdpMux(conn net.PacketConn) *ice.UniversalUDPMuxDefault {
	loggerFactory := logging.NewDefaultLoggerFactory()
	loggerFactory.DefaultLogLevel = logging.LogLevelDebug

	universalUdpMux := ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
		Logger:  loggerFactory.NewLogger("wrapper"),
		UDPConn: conn,
		Net:     nil,
	})

	return universalUdpMux
}

type AgentParams struct {
	LoggerFacotry     logging.LoggerFactory
	StunUrl           string
	UdpMux            *ice.UDPMuxDefault
	UniversalUdpMux   *ice.UniversalUDPMuxDefault
	OnCandidate       func(c ice.Candidate)
	TieBreaker        uint32
	Ufrag             string
	Pwd               string
	KeepaliveInterval time.Duration
}

func NewAgentParams(stunUri, ufrag, pwd string, univeralUdpMux *ice.UniversalUDPMuxDefault, tieBreaker uint32) *AgentParams {
	if stunUri == "" {
		stunUri = "stun:81.68.109.143:3478"
	}

	f := logging.NewDefaultLoggerFactory()
	f.DefaultLogLevel = logging.LogLevelDebug
	return &AgentParams{
		LoggerFacotry:     f,
		StunUrl:           stunUri,
		UdpMux:            univeralUdpMux.UDPMuxDefault,
		UniversalUdpMux:   univeralUdpMux,
		Ufrag:             ufrag,
		Pwd:               pwd,
		TieBreaker:        tieBreaker,
		KeepaliveInterval: 20 * time.Second,
	}
}

// NewAgent creates a new ICE agent, will use to gather candidates
// each peer will create an agent for connection establishment
func NewAgent(params *AgentParams) (*ice.Agent, error) {
	var err error
	var agent *ice.Agent

	var stunUri []*stun.URI
	uri, err := stun.ParseURI(params.StunUrl)
	if err != nil {
		return nil, err
	}
	uri.Username = "admin"
	uri.Password = "admin"
	stunUri = append(stunUri, uri)
	f := logging.NewDefaultLoggerFactory()
	f.DefaultLogLevel = logging.LogLevelDebug
	agent, err = ice.NewAgent(&ice.AgentConfig{
		NetworkTypes:   []ice.NetworkType{ice.NetworkTypeUDP4},
		UDPMux:         params.UdpMux,
		UDPMuxSrflx:    params.UniversalUdpMux,
		Urls:           stunUri,
		Tiebreaker:     uint64(params.TieBreaker),
		LoggerFactory:  f,
		CandidateTypes: []ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypeServerReflexive},
		LocalUfrag:     params.Ufrag,
		LocalPwd:       params.Pwd,
	})

	if err != nil {
		return nil, err
	}

	if params.OnCandidate != nil { // if OnCandidate is nil, we don't need to gather candidates
		if err = agent.OnCandidate(params.OnCandidate); err != nil {
			return nil, err
		}

		if err = agent.GatherCandidates(); err != nil {
			return nil, err
		}
	}

	return agent, err
}

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

func GenerateUfragPwd() (string, string, error) {
	pwd, err := randutil.GenerateCryptoRandomString(lenPwd, runesAlpha)
	if err != nil {
		return "", "", err
	}

	ufrag, err := randutil.GenerateCryptoRandomString(lenUFrag, runesAlpha)
	if err != nil {
		return "", "", err
	}

	return ufrag, pwd, err
}
