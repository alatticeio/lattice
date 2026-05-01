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

package relay

import (
	"errors"
	"sync"

	quic "github.com/quic-go/quic-go"
)

type SessionManager struct {
	mu        sync.RWMutex
	sessions  map[uint64]*Session
	quicConns map[uint64]*quic.Conn
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:  make(map[uint64]*Session),
		quicConns: make(map[uint64]*quic.Conn),
	}
}

func (m *SessionManager) Register(id uint64, s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = s
}

func (m *SessionManager) Unregister(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	delete(m.quicConns, id)
}

func (m *SessionManager) RegisterQUIC(id uint64, ctrl Stream, conn *quic.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = &Session{
		ID:     id,
		Stream: ctrl,
		Type:   "QUIC",
	}
	m.quicConns[id] = conn
}

func (m *SessionManager) Get(id uint64) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

func (m *SessionManager) Relay(toID uint64, frame []byte) error {
	m.mu.RLock()
	qconn := m.quicConns[toID]
	session := m.sessions[toID]
	m.mu.RUnlock()

	if qconn != nil {
		return qconn.SendDatagram(frame)
	}
	if session != nil {
		_, err := session.Stream.Write(frame)
		return err
	}
	return errors.New("lrp: relay target not found")
}

func (m *SessionManager) ConnectedPeers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
