// Copyright 2026 The Lattice Authors, Inc.
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

package nats

import (
	"sync"
	"time"
)

// offlineThreshold is how long since the last heartbeat before a node is considered offline.
// Heartbeat interval is 30s, so 3 missed heartbeats = offline.
const offlineThreshold = 90 * time.Second

// NodePresenceStore is a thread-safe in-memory store that tracks the last
// heartbeat timestamp for each agent node identified by its AppID.
type NodePresenceStore struct {
	mu sync.RWMutex
	m  map[string]time.Time // appId -> lastHeartbeat
}

// NewNodePresenceStore creates an empty NodePresenceStore.
func NewNodePresenceStore() *NodePresenceStore {
	return &NodePresenceStore{
		m: make(map[string]time.Time),
	}
}

// Update records a heartbeat for the given appId at the current time.
func (s *NodePresenceStore) Update(appId string) {
	s.mu.Lock()
	s.m[appId] = time.Now()
	s.mu.Unlock()
}

// GetStatus returns the online status and last-seen time for the given appId.
//
// Possible status values:
//   - "online"  — heartbeat received within the last 90 seconds
//   - "offline" — heartbeat was received before, but longer than 90 seconds ago
//   - "pending" — no heartbeat ever received (node registered but never connected)
func (s *NodePresenceStore) GetStatus(appId string) (status string, lastSeen *time.Time) {
	s.mu.RLock()
	t, ok := s.m[appId]
	s.mu.RUnlock()

	if !ok {
		return "pending", nil
	}

	if time.Since(t) < offlineThreshold {
		return "online", &t
	}
	return "offline", &t
}
