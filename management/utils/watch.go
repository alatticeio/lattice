package utils

import (
	"encoding/json"
	"fmt"
	"linkany/management/entity"
	"linkany/management/grpc/mgt"
	"linkany/pkg/log"
	"sync"
)

var lock sync.Mutex
var once sync.Once
var manager *WatchManager

// NewWatchMessage creates a new WatchMessage, when a peer is added, updated or deleted
func NewWatchMessage(eventType mgt.EventType, peers []*entity.Peer) *mgt.WatchMessage {
	body, err := json.Marshal(peers)
	if err != nil {
		return nil
	}
	return &mgt.WatchMessage{
		Type: eventType,
		Body: body,
	}
}

type WatchManager struct {
	lock   sync.Mutex
	m      map[string]chan *mgt.WatchMessage
	logger *log.Logger
}

// NewWatchManager create a whole manager for connected peers
func NewWatchManager() *WatchManager {
	lock.Lock()
	defer lock.Unlock()
	if manager != nil {
		return manager
	}
	once.Do(func() {
		manager = &WatchManager{
			m:      make(map[string]chan *mgt.WatchMessage),
			logger: log.NewLogger(log.LogLevelVerbose, fmt.Sprintf("[%s] ", "watchmanager")),
		}
	})

	return manager
}

// Add adds a new channel to the watch manager for a new connected peer
func (w *WatchManager) Add(key string, ch chan *mgt.WatchMessage) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.logger.Verbosef("manager: %v, ch: %v", w, ch)
	w.m[key] = ch
}

// Remove removes a channel from the watch manager for a disconnected peer
func (w *WatchManager) Remove(key string) {
	w.lock.Lock()
	defer w.lock.Unlock()

	delete(w.m, key)
}

// Send sends a message to all connected peer's channel
func (w *WatchManager) Send(key string, msg *mgt.WatchMessage) {
	w.lock.Lock()
	defer w.lock.Unlock()

	if ch, ok := w.m[key]; ok {
		ch <- msg
	}
}

func (w *WatchManager) Get(key string) chan *mgt.WatchMessage {
	w.lock.Lock()
	defer w.lock.Unlock()
	ch := w.m[key]
	w.logger.Verbosef("Get channel: %v, manager: %v", ch, w)
	return ch
}
