package internal

import (
	"sync"
)

type NodeManager struct {
	lock  sync.Mutex
	peers map[string]*NodeMessage
}

func NewNodeManager() *NodeManager {
	return &NodeManager{
		peers: make(map[string]*NodeMessage),
	}
}

func (p *NodeManager) AddPeer(key string, peer *NodeMessage) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.peers[key] = peer
}

func (p *NodeManager) GetPeer(key string) *NodeMessage {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.peers[key]
}

func (p *NodeManager) Remove(key string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	delete(p.peers, key)
}
