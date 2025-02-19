package server

import (
	"k8s.io/klog/v2"
	"linkany/pkg/linkerrors"
	"sync"
)

type ForwardManager struct {
	lock sync.Locker
	m    map[string]chan *ForwardMessage
}

type ForwardMessage struct {
	Body []byte
}

func NewForwardManager() *ForwardManager {
	return &ForwardManager{
		lock: &sync.Mutex{},
		m:    make(map[string]chan *ForwardMessage),
	}
}

func (f *ForwardManager) CreateChannel(pubKey string) chan *ForwardMessage {
	f.lock.Lock()
	defer f.lock.Unlock()
	if _, ok := f.m[pubKey]; !ok {
		f.m[pubKey] = make(chan *ForwardMessage, 1000)
	}
	klog.Infof("create channel for %v success", pubKey)
	return f.m[pubKey]
}

func (f *ForwardManager) GetChannel(pubKey string) (chan *ForwardMessage, bool) {
	f.lock.Lock()
	defer f.lock.Unlock()
	ch, ok := f.m[pubKey]
	return ch, ok
}

func (f *ForwardManager) DeleteChannel(pubKey string) {
	f.lock.Lock()
	defer f.lock.Unlock()
	delete(f.m, pubKey)
}

func (f *ForwardManager) ForwardMessage(pubKey string, msg *ForwardMessage) error {
	ch, ok := f.GetChannel(pubKey)
	if !ok {
		return linkerrors.ErrChannelNotExists
	}
	ch <- msg
	return nil
}
