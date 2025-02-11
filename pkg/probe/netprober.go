package probe

import (
	"linkany/internal"
	"linkany/pkg/iface"
	"sync"
)

type NetProber struct {
	lock         sync.Mutex
	probers      map[string]*Prober
	wgLock       sync.Mutex
	isForceRelay bool
	wgConfiger   *iface.WGConfigure
	relayer      internal.Relay
}

func (pm *NetProber) AddProber(key string, prober *Prober) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.probers[key] = prober
}

func (pm *NetProber) GetProber(key string) *Prober {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	return pm.probers[key]
}

func NewProberManager(isForceRelay bool, relayer internal.Relay) *NetProber {
	return &NetProber{
		probers:      make(map[string]*Prober),
		isForceRelay: isForceRelay,
		relayer:      relayer,
	}
}

func (pm *NetProber) RemoveProber(key string) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	delete(pm.probers, key)
}

func (pm *NetProber) SetWgConfiger(wgConfiger *iface.WGConfigure) {
	pm.wgLock.Lock()
	defer pm.wgLock.Unlock()
	pm.wgConfiger = wgConfiger
}

func (pm *NetProber) GetWgConfiger() *iface.WGConfigure {
	pm.wgLock.Lock()
	defer pm.wgLock.Unlock()
	return pm.wgConfiger
}

func (pm *NetProber) IsForceRelay() bool {
	return pm.isForceRelay
}

func (pm *NetProber) SetRelayer(relayer internal.Relay) {
	pm.wgLock.Lock()
	defer pm.wgLock.Unlock()
	pm.relayer = relayer
}
