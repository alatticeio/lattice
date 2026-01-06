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
	"context"
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Client is the interface for managing WireGuard devices.
type Client interface {
	// Start the engine
	Start() error

	GetDeviceName() string

	Configure(peerId string) error

	// Stop the engine
	Stop() error

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

type ManagementClient interface {
	GetNetMap() (*Message, error)
	Register(ctx context.Context, interfaceName string) (*Peer, error)
	AddPeer(p *Peer) error
}

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
