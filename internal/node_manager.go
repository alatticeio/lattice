package internal

import (
	"sync"
)

type NodeManager struct {
	lock  sync.Mutex
	nodes map[string]*Node
}

func NewNodeManager() *NodeManager {
	return &NodeManager{
		nodes: make(map[string]*Node),
	}
}

func (nm *NodeManager) AddPeer(key string, peer *Node) {
	nm.lock.Lock()
	defer nm.lock.Unlock()
	nm.nodes[key] = peer
}

func (nm *NodeManager) GetPeer(key string) *Node {
	nm.lock.Lock()
	defer nm.lock.Unlock()
	return nm.nodes[key]
}

func (nm *NodeManager) Remove(key string) {
	nm.lock.Lock()
	defer nm.lock.Unlock()
	delete(nm.nodes, key)
}
