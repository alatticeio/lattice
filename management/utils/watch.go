package utils

import (
	"linkany/management/grpc/mgt"
	"sync"
)

var lock sync.Mutex
var once sync.Once
var manager *WatchManager

// WatchMessage is the message sent to the chan when a watch event is triggered,
// it contains the event type and the peer that was updated, will seed to every client
type WatchMessage struct {
	// The key of the updated object
	Type mgt.EventType `json:"event_type"`
	Peer *mgt.Peer     `json:"peer"`
}

// NewWatchMessage creates a new WatchMessage, when a peer is added, updated or deleted
func NewWatchMessage(eventType mgt.EventType, peer *mgt.Peer) *WatchMessage {
	return &WatchMessage{
		Type: eventType,
		Peer: peer,
	}
}

type WatchManager struct {
	lock sync.Mutex
	m    map[string]chan *WatchMessage
}

// NewWatchManager create a whole manager for connected peers
func NewWatchManager() *WatchManager {
	defer lock.Unlock()
	lock.Lock()
	if manager != nil {
		return manager
	}
	return &WatchManager{
		m: make(map[string]chan *WatchMessage),
	}
}

// Add adds a new channel to the watch manager for a new connected peer
func (w *WatchManager) Add(key string, ch chan *WatchMessage) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.m[key] = ch
}

// Remove removes a channel from the watch manager for a disconnected peer
func (w *WatchManager) Remove(key string) {
	w.lock.Lock()
	defer w.lock.Unlock()

	delete(w.m, key)
}

// Send sends a message to all connected peer's channel
func (w *WatchManager) Send(key string, msg *WatchMessage) {
	w.lock.Lock()
	defer w.lock.Unlock()

	if ch, ok := w.m[key]; ok {
		ch <- msg
	}
}

func (w *WatchManager) Get(key string) chan *WatchMessage {
	w.lock.Lock()
	defer w.lock.Unlock()
	return w.m[key]
}
