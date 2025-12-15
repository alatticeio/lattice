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

package domain

import (
	"sync"
)

type NodeChannel struct {
	nu        sync.Mutex
	NetworkId []string
	Channel   chan *Message // key: clientId, value: Channel
}

func (n *NodeChannel) GetChannel() chan *Message {
	n.nu.Lock()
	defer n.nu.Unlock()
	if n.Channel == nil {
		n.Channel = make(chan *Message, 1000) // buffered Channel
	}
	return n.Channel
}

// IWatchManager used to manage all the channels of connected peers
type IWatchManager interface {
	GetChannel(clientId string) *NodeChannel
	Remove(clientID string)
	Send(clientId string, msg *Message) error
}
