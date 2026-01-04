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

package manager

import (
	"sync"
	"wireflow/internal/core/domain"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	_ domain.KeyManager = (*keyManager)(nil)
)

type keyManager struct {
	lock       sync.Mutex
	privateKey string
}

func NewKeyManager(privateKey string) domain.KeyManager {
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
