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

package infra

import (
	"encoding/binary"
	"fmt"
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type PeerManager struct {
	mu    sync.RWMutex
	peers map[string]*Peer
}

func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers: make(map[string]*Peer),
	}
}

func (p *PeerManager) AddPeer(peerId string, peer *Peer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers[peerId] = peer
}

func (p *PeerManager) RemovePeer(peerId string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.peers, peerId)
}

func (p *PeerManager) GetPeer(peerId string) *Peer {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.peers[peerId]
}

// PeerID union of public key and peer ID
type PeerID [8]byte

func FromKey(key wgtypes.Key) PeerID {
	var id PeerID
	copy(id[:], key[:8])
	return id
}

func (p PeerID) ToUint64() uint64 {
	return binary.BigEndian.Uint64(p[:])
}

func FromUint64(id uint64) PeerID {
	var peerID PeerID
	binary.BigEndian.PutUint64(peerID[:], id)
	return peerID
}

func (id PeerID) String() string {
	return fmt.Sprintf("%d", id.ToUint64())
}

type PeerEntry struct {
	PublicKey wgtypes.Key
	ID        PeerID
}

type PeerStore struct {
	mu     sync.RWMutex
	idMap  map[PeerID]*PeerEntry
	keyMap map[wgtypes.Key]*PeerEntry
}

func NewPeerStore() *PeerStore {
	return &PeerStore{
		idMap:  make(map[PeerID]*PeerEntry),
		keyMap: make(map[wgtypes.Key]*PeerEntry),
	}
}

// AddPeer 注册一个新的邻居。在 NATS 握手完成后调用。
func (s *PeerStore) AddPeer(key wgtypes.Key) *PeerEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := FromKey(key)
	entry := &PeerEntry{
		PublicKey: key,
		ID:        id,
	}

	s.idMap[id] = entry
	s.keyMap[key] = entry
	return entry
}

// GetKeyByID 实现你要求的“反查”逻辑
func (s *PeerStore) GetKeyByID(id PeerID) (wgtypes.Key, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.idMap[id]
	if !ok {
		return wgtypes.Key{}, false
	}
	return entry.PublicKey, true
}
