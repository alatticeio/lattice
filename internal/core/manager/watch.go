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
	"wireflow/internal/log"
)

var lock sync.Mutex
var once sync.Once
var manager *WatchManager

//var (
//	_ domain.IWatchManager = (*WatchManager)(nil)
//)

// WatchManager is a singleton that manages the watch channels for connected peers
// It is used to send messages to all connected peers
// watcher is a map of networkId to watcher, a watcher is a struct that contains the networkId
// and the channel to send messages to
// m is a map of groupId_nodeId to channel, a channel is used to send messages to the connected peer
// The key is a combination of networkId and publicKey, which is used to identify the connected peer
type WatchManager struct {
	mu sync.Mutex
	// push channel
	channels     map[string]*domain.NodeChannel // key: clientId, value: channel
	recvChannels map[string]*domain.NodeChannel
	logger       *log.Logger
}

// GetChannel get channel by clientID`
func (w *WatchManager) GetChannel(clientId string) *domain.NodeChannel {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.channels == nil {
		return nil
	}
	channel := w.channels[clientId]

	if channel == nil {
		channel = &domain.NodeChannel{
			Channel: make(chan *domain.Message, 1000), // buffered channel
		}
	}
	w.channels[clientId] = channel
	return channel
}

func (w *WatchManager) Remove(clientID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	channel := w.channels[clientID]
	if channel == nil {
		w.logger.Errorf("channel not found for clientID: %s", clientID)
		return
	}

	delete(w.channels, clientID)
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
			channels: make(map[string]*domain.NodeChannel),
			logger:   log.NewLogger(log.Loglevel, "watchmanager"),
		}
	})

	return manager
}

func (w *WatchManager) Send(clientId string, msg *domain.Message) error {
	channel := w.GetChannel(clientId)
	channel.Channel <- msg
	return nil
}
