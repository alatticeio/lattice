// internal/server/transport/state_machine_test.go
package transport

import (
	"sync"
	"testing"
)

func TestStateMachine_ValidTransitions(t *testing.T) {
	tests := []struct {
		name    string
		current PeerState
		target  PeerState
		wantErr bool
	}{
		// Allowed transitions
		{"created→probing", StateCreated, StateProbing, false},
		{"probing→ice-ready", StateProbing, StateICEReady, false},
		{"probing→wrrp-ready", StateProbing, StateWRRPReady, false},
		{"probing→failed", StateProbing, StateFailed, false},
		{"wrrp-ready→ice-ready", StateWRRPReady, StateICEReady, false},
		{"wrrp-ready→failed", StateWRRPReady, StateFailed, false},
		{"wrrp-ready→closed", StateWRRPReady, StateClosed, false},
		{"ice-ready→failed", StateICEReady, StateFailed, false},
		{"ice-ready→closed", StateICEReady, StateClosed, false},
		{"failed→probing", StateFailed, StateProbing, false},
		{"failed→closed", StateFailed, StateClosed, false},
		// Invalid transitions
		{"created→ice-ready", StateCreated, StateICEReady, true},
		{"created→failed", StateCreated, StateFailed, true},
		{"ice-ready→probing", StateICEReady, StateProbing, true},
		{"ice-ready→wrrp-ready", StateICEReady, StateWRRPReady, true},
		{"closed→probing", StateClosed, StateProbing, true},
		{"closed→ice-ready", StateClosed, StateICEReady, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewStateMachine(tt.current)
			err := sm.Transition(tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transition() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if got := sm.Current(); got != tt.target {
					t.Errorf("Current() = %v, want %v", got, tt.target)
				}
			}
		})
	}
}

func TestStateMachine_OnTransition(t *testing.T) {
	var mu sync.Mutex
	var calls []struct{ from, to PeerState }
	sm := NewStateMachine(StateCreated)
	sm.OnTransition(func(from, to PeerState) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, struct{ from, to PeerState }{from, to})
	})

	_ = sm.Transition(StateProbing)
	_ = sm.Transition(StateICEReady)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected 2 callbacks, got %d", len(calls))
	}
	if calls[0].from != StateCreated || calls[0].to != StateProbing {
		t.Errorf("first call: from=%s to=%s", calls[0].from, calls[0].to)
	}
	if calls[1].from != StateProbing || calls[1].to != StateICEReady {
		t.Errorf("second call: from=%s to=%s", calls[1].from, calls[1].to)
	}
}

func TestStateMachine_TransitionFailed_NoCallback(t *testing.T) {
	called := false
	sm := NewStateMachine(StateCreated)
	sm.OnTransition(func(from, to PeerState) {
		called = true
	})

	err := sm.Transition(StateICEReady) // invalid from created
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
	if called {
		t.Fatal("OnTransition should not be called on failed transition")
	}
}

func TestStateMachine_ConcurrentTransitions(t *testing.T) {
	sm := NewStateMachine(StateCreated)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sm.Transition(StateProbing)
		}()
	}
	wg.Wait()
	// State should be either StateCreated or StateProbing (never corrupted)
	current := sm.Current()
	if current != StateCreated && current != StateProbing {
		t.Errorf("unexpected state after concurrent transitions: %s", current)
	}
}
