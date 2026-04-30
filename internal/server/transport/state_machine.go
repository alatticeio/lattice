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
	"fmt"
	"sync"

	victoriametrics "github.com/VictoriaMetrics/metrics"
)

// PeerState represents the lifecycle stage of a peer connection.
type PeerState string

const (
	StateCreated   PeerState = "created"
	StateProbing   PeerState = "probing"
	StateICEReady  PeerState = "ice-ready"
	StateWRRPReady PeerState = "wrrp-ready"
	StateFailed    PeerState = "failed"
	StateClosed    PeerState = "closed"
)

func (s PeerState) String() string { return string(s) }

// State machine transition counter — exposed via VictoriaMetrics global set.
// Metric: lattice_transport_state_changes_total{from="probing",to="ice-ready"}
var stateChangeCounter = victoriametrics.NewCounter(`lattice_transport_state_changes_total`)

// allowedTransitions defines the legal state transitions.
var allowedTransitions = map[PeerState][]PeerState{
	StateCreated:   {StateProbing},
	StateProbing:   {StateICEReady, StateWRRPReady, StateFailed},
	StateWRRPReady: {StateICEReady, StateFailed, StateClosed},
	StateICEReady:  {StateFailed, StateClosed},
	StateFailed:    {StateProbing, StateClosed},
}

// StateMachine guards connection lifecycle transitions.
// It does NOT handle side effects — callers register callbacks via OnTransition.
type StateMachine struct {
	mu        sync.Mutex
	current   PeerState
	callbacks []func(from, to PeerState)
}

// NewStateMachine creates a state machine in the given initial state.
func NewStateMachine(initial PeerState) *StateMachine {
	return &StateMachine{current: initial}
}

// Transition attempts to move from the current state to target.
// Returns an error if the transition is not allowed.
// On success, fires all registered OnTransition callbacks.
func (sm *StateMachine) Transition(target PeerState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	allowed := allowedTransitions[sm.current]
	for _, t := range allowed {
		if t == target {
			from := sm.current
			sm.current = target
			stateChangeCounter.Inc()
			for _, cb := range sm.callbacks {
				cb(from, target)
			}
			return nil
		}
	}
	return fmt.Errorf("invalid transition %s → %s", sm.current, target)
}

// Current returns the current state (thread-safe).
func (sm *StateMachine) Current() PeerState {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.current
}

// OnTransition registers a callback that fires on every successful transition.
func (sm *StateMachine) OnTransition(cb func(from, to PeerState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.callbacks = append(sm.callbacks, cb)
}
