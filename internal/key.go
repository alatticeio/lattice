package internal

import (
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"sync"
)

type KeyManager struct {
	lock       sync.Mutex
	privateKey string
}

func NewKeyManager(privateKey string) *KeyManager {
	return &KeyManager{privateKey: privateKey}
}

func (km *KeyManager) UpdateKey(privateKey string) {
	km.lock.Lock()
	defer km.lock.Unlock()
	km.privateKey = privateKey
}

func (km *KeyManager) GetKey() string {
	km.lock.Lock()
	defer km.lock.Unlock()
	return km.privateKey
}

func (km *KeyManager) GetPublicKey() string {
	km.lock.Lock()
	defer km.lock.Unlock()
	key, err := wgtypes.ParseKey(km.privateKey)
	if err != nil {
		return ""
	}
	return key.PublicKey().String()
}
