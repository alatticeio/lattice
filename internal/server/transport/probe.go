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
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/grpc"

	"github.com/pion/ice/v4"
)

var (
	_ infra.Probe = (*Probe)(nil)
)

// Probe manages the connection lifecycle to a single remote peer.
type Probe struct {
	mu         sync.RWMutex
	localId    infra.PeerIdentity
	remoteId   infra.PeerIdentity
	iceDialer  infra.Dialer
	wrrpDialer infra.Dialer
	iceState   ice.ConnectionState
	signal     infra.SignalService
	log        *log.Logger

	// State machine guards lifecycle transitions.
	sm *StateMachine

	// Configurator handles WireGuard side-effects (peer, route, NAT).
	configurator ConnectionConfigurator

	// Factory funcs for creating fresh dialers on restart.
	newIceDialer  func() infra.Dialer
	newWrrpDialer func() infra.Dialer

	// onBeforeRestart is called before rebuilding dialers to clean up
	// stale WireGuard peer state.
	onBeforeRestart func()

	// Epoch and running for discover() goroutine coordination.
	epoch   atomic.Uint64
	running atomic.Bool

	// currentTransport holds the active transport.
	currentTransport infra.Transport

	// Remote peer identity received from SYN/ACK.
	// firstFailureAt tracks consecutive failure duration for 60s timeout.
	muFail         sync.Mutex
	firstFailureAt time.Time
}

func (p *Probe) Handle(ctx context.Context, remoteId infra.PeerIdentity, packet *grpc.SignalPacket) error {
	switch packet.Dialer {
	case grpc.DialerType_ICE:
		p.mu.RLock()
		d := p.iceDialer
		p.mu.RUnlock()
		return d.Handle(ctx, p.remoteId, packet)
	case grpc.DialerType_WRRP:
		p.mu.RLock()
		d := p.wrrpDialer
		p.mu.RUnlock()
		return d.Handle(ctx, p.remoteId, packet)
	}
	return nil
}

// restart replaces both dialers with fresh instances and re-runs discovery.
func (p *Probe) restart() {
	if p.newIceDialer == nil {
		return
	}
	// Clean up stale WireGuard peer state.
	if p.onBeforeRestart != nil {
		p.onBeforeRestart()
	}
	p.mu.Lock()
	p.iceDialer = p.newIceDialer()
	if p.newWrrpDialer != nil {
		p.wrrpDialer = p.newWrrpDialer()
	}
	p.mu.Unlock()

	p.epoch.Add(1)
	p.running.Store(false)
	_ = p.Start(context.Background(), p.remoteId)
}

// Close permanently stops this probe.
func (p *Probe) Close() {
	p.mu.Lock()
	p.newIceDialer = nil
	p.newWrrpDialer = nil
	d := p.iceDialer
	p.iceDialer = nil
	wd := p.wrrpDialer
	p.wrrpDialer = nil
	p.mu.Unlock()

	if d != nil {
		d.Close() //nolint:errcheck
	}
	if wd != nil {
		wd.Close() //nolint:errcheck
	}
}

func (p *Probe) OnConnectionStateChange(state ice.ConnectionState) {
	p.mu.Lock()
	p.iceState = state
	p.mu.Unlock()
	p.log.Debug("Setting new connection status", "state", state)
}

func (p *Probe) Start(ctx context.Context, remoteId infra.PeerIdentity) error {
	if !p.running.CompareAndSwap(false, true) {
		p.log.Warn("Probe already started")
		return nil
	}

	myEpoch := p.epoch.Load()
	p.log.Debug("Start probe peer", "localId", p.localId, "remoteId", remoteId)

	// Transition to Probing (valid from Created or Failed).
	_ = p.sm.Transition(StateProbing)

	go func() {
		t, err := p.discover(ctx)

		if p.epoch.Load() != myEpoch {
			if t != nil {
				t.Close() //nolint:errcheck
			}
			return
		}
		p.running.Store(false)

		if err != nil {
			p.log.Error("Discover transport failed", err)
			p.onFailure(err)
			return
		}

		p.onSuccess(t)
	}()

	return nil
}

func (p *Probe) Ping(ctx context.Context) error {
	return nil
}

// onSuccess handles the first successful transport connection.
func (p *Probe) onSuccess(transport infra.Transport) {
	p.mu.Lock()
	p.currentTransport = transport
	p.mu.Unlock()

	transportType := transport.Type()
	if transportType == infra.ICE {
		_ = p.sm.Transition(StateICEReady)
	} else {
		_ = p.sm.Transition(StateWRRPReady)
	}
}

// onFailure handles discovery failure.
func (p *Probe) onFailure(err error) {
	// ErrDialerClosed: clean session transition, restart immediately.
	if errors.Is(err, ErrDialerClosed) {
		p.muFail.Lock()
		p.firstFailureAt = time.Time{}
		p.muFail.Unlock()
		_ = p.sm.Transition(StateFailed)
		p.restart()
		return
	}

	p.muFail.Lock()
	if p.firstFailureAt.IsZero() {
		p.firstFailureAt = time.Now()
	}
	elapsed := time.Since(p.firstFailureAt)
	p.muFail.Unlock()

	if elapsed >= 60*time.Second {
		p.log.Info("peer unreachable for 60s, closing probe", "remoteId", p.remoteId.AppID)
		_ = p.sm.Transition(StateClosed)
		// Factory handles probe removal externally.
		return
	}

	p.log.Warn("discover failed, retrying in 10s", "remoteId", p.remoteId.AppID, "err", err)
	_ = p.sm.Transition(StateFailed)
	time.AfterFunc(10*time.Second, p.restart)
}

// discover races ICE and WRRP dialers concurrently.
func (p *Probe) discover(ctx context.Context) (infra.Transport, error) {
	dialerCount := 1
	if config.Conf.EnableWrrp {
		dialerCount = 2
	}

	result := make(chan infra.Transport, dialerCount)
	errs := make(chan error, dialerCount)
	var wrrpWon atomic.Bool

	go func() {
		p.log.Debug("Starting ice dialer", "remoteId", p.remoteId)
		if err := p.iceDialer.Prepare(ctx, p.remoteId); err != nil {
			p.log.Error("Prepare failed", err)
			errs <- err
			return
		}
		t, err := p.iceDialer.Dial(ctx)
		if err != nil {
			errs <- err
			return
		}
		result <- t
		if wrrpWon.Load() {
			if err = p.handleUpgradeTransport(t); err != nil {
				p.log.Error("Upgrade transport failed", err)
			}
		}
	}()

	if config.Conf.EnableWrrp {
		go func() {
			p.log.Debug("Starting wrrp dialer", "remoteId", p.remoteId)
			if err := p.wrrpDialer.Prepare(ctx, p.remoteId); err != nil {
				errs <- err
				return
			}
			t, err := p.wrrpDialer.Dial(ctx)
			if err != nil {
				errs <- err
				return
			}
			result <- t
		}()
	}

	failed := 0
	var lastErr error
	for {
		select {
		case t := <-result:
			if t.Type() == infra.WRRP && config.Conf.EnableWrrp {
				select {
				case iceT := <-result:
					_ = t.Close()
					return iceT, nil
				case <-time.After(500 * time.Millisecond):
					wrrpWon.Store(true)
				}
			}
			return t, nil
		case err := <-errs:
			lastErr = err
			failed++
			if failed == dialerCount {
				return nil, lastErr
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (p *Probe) handleUpgradeTransport(newTransport infra.Transport) error {
	p.log.Debug("Upgrade transport....", "newTransport", newTransport)
	p.mu.Lock()
	old := p.currentTransport
	p.currentTransport = newTransport
	p.mu.Unlock()

	// Close old after delay.
	if old != nil {
		go func() {
			time.Sleep(2 * time.Second)
			old.Close() //nolint:errcheck
		}()
	}

	// Transition WRRPReady -> ICEReady: WG config handled by state machine callbacks.
	_ = p.sm.Transition(StateICEReady)
	return nil
}
