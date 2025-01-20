package config

import (
	"sync"
)

type PeersManager struct {
	lock  sync.Mutex
	peers map[string]*Peer
}

func NewPeersManager() *PeersManager {
	return &PeersManager{
		peers: make(map[string]*Peer),
	}
}

func (p *PeersManager) AddPeer(key string, peer *Peer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.peers[key] = peer
}

func (p *PeersManager) GetPeer(key string) *Peer {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.peers[key]
}

func (p *PeersManager) Remove(key string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	delete(p.peers, key)
}
