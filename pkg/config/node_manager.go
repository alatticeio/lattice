package config

import (
	"linkany/management/utils"
	"linkany/pkg/log"
	"sync"
)

type NodeManager struct {
	logger *log.Logger
	lock   sync.Mutex
	peers  map[string]*utils.NodeMessage
}

func NewPeersManager() *NodeManager {
	return &NodeManager{
		logger: log.NewLogger(log.Loglevel, "node-manager"),
		peers:  make(map[string]*utils.NodeMessage),
	}
}

func (p *NodeManager) AddPeer(key string, peer *utils.NodeMessage) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.peers[key] = peer
	p.logger.Verbosef("Add node for key: %v, node: %v", key, peer)
}

func (p *NodeManager) GetPeer(key string) *utils.NodeMessage {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.peers[key]
}

func (p *NodeManager) Remove(key string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	delete(p.peers, key)
}
