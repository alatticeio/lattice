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
	"net"
	"sync"
	"testing"
)

type mockStream struct {
	written [][]byte
}

func (m *mockStream) Read(p []byte) (int, error)  { return 0, nil }
func (m *mockStream) Write(p []byte) (int, error) { m.written = append(m.written, p); return len(p), nil }
func (m *mockStream) Close() error                { return nil }
func (m *mockStream) RemoteAddr() net.Addr        { return nil }

func TestSessionManager_RegisterAndGet(t *testing.T) {
	sm := NewSessionManager()
	stream := &mockStream{}
	sm.Register(42, &Session{ID: 42, Stream: stream, Type: "TCP"})

	s := sm.Get(42)
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.ID != 42 {
		t.Errorf("expected ID 42, got %d", s.ID)
	}
}

func TestSessionManager_Unregister(t *testing.T) {
	sm := NewSessionManager()
	sm.Register(42, &Session{ID: 42, Stream: &mockStream{}, Type: "TCP"})
	sm.Unregister(42)

	if sm.Get(42) != nil {
		t.Error("session should be nil after unregister")
	}
}

func TestSessionManager_RelayToMissingTarget(t *testing.T) {
	sm := NewSessionManager()
	err := sm.Relay(99, []byte("data"))
	if err == nil {
		t.Error("expected error when relaying to missing target")
	}
}

func TestSessionManager_RelayTCP(t *testing.T) {
	sm := NewSessionManager()
	stream := &mockStream{}
	sm.Register(99, &Session{ID: 99, Stream: stream, Type: "TCP"})

	err := sm.Relay(99, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stream.written) != 1 {
		t.Fatalf("expected 1 write, got %d", len(stream.written))
	}
	if string(stream.written[0]) != "hello" {
		t.Errorf("unexpected payload: %s", stream.written[0])
	}
}

func TestSessionManager_ConcurrentAccess(t *testing.T) {
	sm := NewSessionManager()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		id := uint64(i)
		go func() { defer wg.Done(); sm.Register(id, &Session{ID: id, Stream: &mockStream{}, Type: "TCP"}) }()
		go func() { defer wg.Done(); sm.Get(id) }()
		go func() { defer wg.Done(); sm.Unregister(id) }()
	}
	wg.Wait()
}

func TestSessionManager_ConnectedPeers(t *testing.T) {
	sm := NewSessionManager()
	if sm.ConnectedPeers() != 0 {
		t.Error("expected 0 connected peers")
	}
	sm.Register(1, &Session{ID: 1, Stream: &mockStream{}, Type: "TCP"})
	sm.Register(2, &Session{ID: 2, Stream: &mockStream{}, Type: "TCP"})
	if sm.ConnectedPeers() != 2 {
		t.Errorf("expected 2 connected peers, got %d", sm.ConnectedPeers())
	}
	sm.Unregister(1)
	if sm.ConnectedPeers() != 1 {
		t.Errorf("expected 1 connected peer after unregister, got %d", sm.ConnectedPeers())
	}
}
