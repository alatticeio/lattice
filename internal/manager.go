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

package internal

import (
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// IClient is the interface for managing WireGuard devices.
type IClient interface {
	// Start the engine
	Start() error

	// Stop the engine
	Stop() error

	// GetDeviceConfiger  // Get the WireGuard configuration manager
	GetDeviceConfiger() Configurer

	Configure(conf *DeviceConfig) error

	// AddPeer adds a peer to the WireGuard device, add peer from contrl client, then will start connect to peer
	AddPeer(peer *Peer) error

	// RemovePeer removes a peer from the WireGuard device
	RemovePeer(peer *Peer) error

	RemoveAllPeers()
}

// KeyManager manage the device keys
type KeyManager interface {
	// UpdateKey updates the private key used for encryption.
	UpdateKey(privateKey string)
	// GetKey retrieves the current private key.
	GetKey() string
	// GetPublicKey retrieves the public key derived from the current private key.
	GetPublicKey() string
}

var (
	_ KeyManager = (*keyManager)(nil)
)

type keyManager struct {
	lock       sync.Mutex
	privateKey string
}

func NewKeyManager(privateKey string) KeyManager {
	return &keyManager{privateKey: privateKey}
}

func (km *keyManager) UpdateKey(privateKey string) {
	km.lock.Lock()
	defer km.lock.Unlock()
	km.privateKey = privateKey
}

func (km *keyManager) GetKey() string {
	km.lock.Lock()
	defer km.lock.Unlock()
	return km.privateKey
}

func (km *keyManager) GetPublicKey() string {
	km.lock.Lock()
	defer km.lock.Unlock()
	key, err := wgtypes.ParseKey(km.privateKey)
	if err != nil {
		return ""
	}
	return key.PublicKey().String()
}

// PeerManager manager all peers connected or connecte to
type PeerManager struct {
	lock  sync.Mutex
	peers map[string]*Peer
}

func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers: make(map[string]*Peer),
	}
}

func (pm *PeerManager) AddPeer(key string, peer *Peer) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.peers[key] = peer
}

func (pm *PeerManager) GetPeer(key string) *Peer {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	return pm.peers[key]
}

func (pm *PeerManager) Remove(key string) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	delete(pm.peers, key)
}
