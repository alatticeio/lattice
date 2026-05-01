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

package transport

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
)

// mockTransport implements infra.Transport for testing.
type mockTransport struct {
	tp     infra.TransportType
	addr   string
	closed bool
}

func (m *mockTransport) Priority() uint8               { return 0 }
func (m *mockTransport) Close() error                  { m.closed = true; return nil }
func (m *mockTransport) Write(data []byte) error       { return nil }
func (m *mockTransport) Read(buff []byte) (int, error) { return 0, nil }
func (m *mockTransport) RemoteAddr() string            { return m.addr }
func (m *mockTransport) Type() infra.TransportType     { return m.tp }

func TestProbe_onSuccess_ICE(t *testing.T) {
	sm := NewStateMachine(StateProbing)
	var mu sync.Mutex
	var transitions []struct{ from, to PeerState }
	sm.OnTransition(func(from, to PeerState) {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, struct{ from, to PeerState }{from, to})
	})

	p := &Probe{sm: sm}
	transport := &mockTransport{tp: infra.ICE, addr: "1.2.3.4:5000"}
	p.onSuccess(transport)

	if got := sm.Current(); got != StateICEReady {
		t.Errorf("expected StateICEReady, got %s", got)
	}
	if p.currentTransport != transport {
		t.Errorf("currentTransport not set")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(transitions) != 1 || transitions[0].to != StateICEReady {
		t.Errorf("expected transition to ice-ready, got %v", transitions)
	}
}

func TestProbe_onSuccess_WRRP(t *testing.T) {
	sm := NewStateMachine(StateProbing)
	p := &Probe{sm: sm}
	transport := &mockTransport{tp: infra.WRRP, addr: "fake"}
	p.onSuccess(transport)

	if got := sm.Current(); got != StateWRRPReady {
		t.Errorf("expected StateWRRPReady, got %s", got)
	}
}

func TestProbe_handleUpgradeTransport(t *testing.T) {
	sm := NewStateMachine(StateWRRPReady)
	p := &Probe{
		sm:  sm,
		log: log.GetLogger("test-probe"),
	}
	newTransport := &mockTransport{tp: infra.ICE, addr: "5.6.7.8:6000"}

	if err := p.handleUpgradeTransport(newTransport); err != nil {
		t.Fatalf("handleUpgradeTransport error: %v", err)
	}

	if got := sm.Current(); got != StateICEReady {
		t.Errorf("expected StateICEReady after upgrade, got %s", got)
	}
	if p.currentTransport != newTransport {
		t.Errorf("currentTransport should be upgraded")
	}
}

func TestProbe_onFailure_ErrDialerClosed_ResetsFailureClock(t *testing.T) {
	sm := NewStateMachine(StateProbing)
	p := &Probe{sm: sm}
	p.muFail.Lock()
	p.firstFailureAt = time.Now()
	p.muFail.Unlock()

	p.onFailure(ErrDialerClosed)

	p.muFail.Lock()
	ft := p.firstFailureAt
	p.muFail.Unlock()

	if !ft.IsZero() {
		t.Errorf("firstFailureAt should be reset to zero on ErrDialerClosed, got %v", ft)
	}
	if got := sm.Current(); got != StateFailed {
		t.Errorf("expected StateFailed, got %s", got)
	}
}

func TestProbe_onFailure_RegularError_SetsFailureClock(t *testing.T) {
	sm := NewStateMachine(StateProbing)
	p := &Probe{
		sm:  sm,
		log: log.GetLogger("test-probe"),
	}

	p.onFailure(errors.New("connection refused"))

	p.muFail.Lock()
	ft := p.firstFailureAt
	p.muFail.Unlock()

	if ft.IsZero() {
		t.Error("firstFailureAt should be set on regular error")
	}
	if got := sm.Current(); got != StateFailed {
		t.Errorf("expected StateFailed, got %s", got)
	}
}

func TestStateMachine_TransitionFromCreated(t *testing.T) {
	sm := NewStateMachine(StateCreated)
	if err := sm.Transition(StateProbing); err != nil {
		t.Fatalf("Created -> Probing should succeed: %v", err)
	}
	// After reaching Probing, ICEReady is valid.
	if err := sm.Transition(StateICEReady); err != nil {
		t.Fatalf("Probing -> ICEReady should succeed: %v", err)
	}

	// Verify that ICEReady cannot go back to Probing.
	sm2 := NewStateMachine(StateICEReady)
	if err := sm2.Transition(StateProbing); err == nil {
		t.Error("ICEReady -> Probing should fail")
	}
}

func TestStateMachine_RejectedTransition_NoCallback(t *testing.T) {
	sm := NewStateMachine(StateCreated)
	called := false
	sm.OnTransition(func(from, to PeerState) {
		called = true
	})

	// Invalid transition should not fire callbacks.
	_ = sm.Transition(StateClosed)
	if called {
		t.Error("OnTransition should not be called on rejected transition")
	}
}
