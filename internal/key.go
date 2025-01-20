package internal

import (
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"sync"
)

type KeyManager struct {
	lock       sync.Mutex
	privateKey wgtypes.Key
}

func NewKeyManager(privateKey wgtypes.Key) *KeyManager {
	return &KeyManager{privateKey: privateKey}
}

func (km *KeyManager) UpdateKey(privateKey wgtypes.Key) {
	km.lock.Lock()
	defer km.lock.Unlock()
	km.privateKey = privateKey
}

func (km *KeyManager) GetKey() wgtypes.Key {
	km.lock.Lock()
	defer km.lock.Unlock()
	return km.privateKey
}

func (km *KeyManager) GetPublicKey() wgtypes.Key {
	km.lock.Lock()
	defer km.lock.Unlock()
	return km.privateKey.PublicKey()
}
