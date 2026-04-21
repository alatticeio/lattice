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

package transport

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
	"wireflow/internal/config"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/wireflowio/ice"
)

var (
	_ infra.Probe = (*Probe)(nil)
)

// Probe for probe connection from two peerManager.
type Probe struct {
	mu        sync.RWMutex
	localId   infra.PeerIdentity
	remoteId  infra.PeerIdentity
	iceDialer infra.Dialer
	state     ice.ConnectionState
	signal    infra.SignalService
	log       *log.Logger
	rtt       time.Duration // nolint

	started atomic.Bool

	// Add wrrp
	wrrpDialer infra.Dialer

	// newIceDialer creates a fresh iceDialer instance for reconnection.
	newIceDialer func() infra.Dialer

	onSuccess        func(transport infra.Transport) error
	onFailure        func(error) error
	currentTransport infra.Transport
}

func (p *Probe) Handle(ctx context.Context, remoteId infra.PeerIdentity, packet *grpc.SignalPacket) error {
	switch packet.Dialer {
	case grpc.DialerType_ICE:
		p.mu.RLock()
		d := p.iceDialer
		p.mu.RUnlock()
		return d.Handle(ctx, p.remoteId, packet)
	case grpc.DialerType_WRRP:
		return p.wrrpDialer.Handle(ctx, p.remoteId, packet)
	}

	return nil
}

// restart replaces the iceDialer with a fresh instance and re-runs discovery.
// Called by the iceDialer's OnClose callback when a connection is lost.
func (p *Probe) restart() {
	if p.newIceDialer == nil {
		return
	}
	p.mu.Lock()
	p.iceDialer = p.newIceDialer()
	p.mu.Unlock()
	p.started.Store(false)
	_ = p.Start(context.Background(), p.remoteId)
}

// Close permanently stops this probe and releases all resources.
// Setting newIceDialer to nil prevents restart() from spinning up a new
// dialer after the underlying iceDialer's close triggers the onClose callback.
func (p *Probe) Close() {
	p.mu.Lock()
	p.newIceDialer = nil
	d := p.iceDialer
	p.iceDialer = nil
	p.mu.Unlock()

	if d != nil {
		d.Close() //nolint:errcheck
	}
}

func (p *Probe) OnConnectionStateChange(state ice.ConnectionState) {
	p.updateState(state)
	p.log.Debug("Setting new connection status", "state", state)
}

func (p *Probe) Start(ctx context.Context, remoteId infra.PeerIdentity) error {
	if p.started.Load() {
		p.log.Warn("Probe already started")
		return nil
	}

	p.started.Store(true)
	p.log.Debug("Start probe peer", "localId", p.localId, "remoteId", remoteId)

	go func() {
		t, err := p.discover(ctx)
		if err != nil {
			p.updateState(ice.ConnectionStateFailed)
			p.log.Error("Discover transport failed", err)
			err = p.onFailure(err)
			if err != nil {
				p.updateState(ice.ConnectionStateFailed)
			}
			return
		}

		p.mu.Lock()
		p.currentTransport = t
		p.mu.Unlock()
		if err = p.onSuccess(t); err != nil {
			p.updateState(ice.ConnectionStateFailed)
		}
	}()

	return nil
}

func (p *Probe) Ping(ctx context.Context) error {
	return nil
}

func (p *Probe) updateState(state ice.ConnectionState) {
	p.state = state
}

// discover races ICE and WRRP dialers concurrently and returns whichever
// transport connects first, with one exception: if WRRP wins the race, it
// waits an extra 500ms to give ICE a chance to catch up.  If ICE arrives
// within that window the WRRP connection is discarded in favour of the
// higher-priority direct path; otherwise WRRP is used immediately and ICE
// can still upgrade later via handleUpgradeTransport.
//
// The select loop collects results and errors:
//   - First successful Transport → return it (after the optional 500ms ICE wait)
//   - All dialers failed → return the last error, caller's onFailure fires
//   - ctx cancelled → propagate the context error
func (p *Probe) discover(ctx context.Context) (infra.Transport, error) {
	dialerCount := 1
	if config.Conf.EnableWrrp {
		dialerCount = 2
	}

	result := make(chan infra.Transport, dialerCount)
	errs := make(chan error, dialerCount)

	// wrrpWon is set to true when WRRP wins the initial race and ICE has not
	// yet arrived within the 500 ms upgrade window.  The ICE goroutine reads
	// this flag to decide whether to call handleUpgradeTransport:
	//   - false (default): ICE was the initial winner; probe.Start() calls
	//     onSuccess exactly once — no second call needed.
	//   - true: WRRP won; ICE arrives later as an upgrade path and must call
	//     handleUpgradeTransport to switch the WireGuard endpoint.
	var wrrpWon atomic.Bool

	// ICE goroutine: completes the full SYN→ACK→OFFER→Dial handshake.
	// Dial blocks until an OFFER is received (up to 65s) or ctx is cancelled.
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
		// Only upgrade when WRRP already owns the active transport.
		// When ICE wins the race discover() returns it directly and
		// probe.Start() calls onSuccess — a second call here would
		// double-apply AddPeer/ApplyRoute/SetupNAT and cause reconnects.
		if wrrpWon.Load() {
			if err = p.handleUpgradeTransport(t); err != nil {
				p.log.Error("Upgrade transport failed", err)
			}
		}
	}()

	// WRRP goroutine: registers with the relay server and establishes a tunnel.
	// Only started when WRRP is enabled; acts as a fallback if ICE cannot
	// complete within the race window.
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

	// Race: first success wins; all failures → error.
	failed := 0
	var lastErr error
	for {
		select {
		case t := <-result:
			// WRRP arrived first: hold on for 500ms to see if ICE can still win.
			// If ICE arrives in time, discard WRRP and return the direct path.
			if t.Type() == infra.WRRP && config.Conf.EnableWrrp {
				select {
				case iceT := <-result:
					_ = t.Close()
					return iceT, nil
				case <-time.After(500 * time.Millisecond):
					// WRRP wins; mark so the ICE goroutine knows to upgrade later.
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
	defer p.mu.Unlock()

	// 权重比较：直连优于中转
	if p.currentTransport == nil || newTransport.Priority() > p.currentTransport.Priority() {
		old := p.currentTransport
		p.currentTransport = newTransport

		// 延迟关闭旧连接，确保缓冲区数据发完
		if old != nil {
			go func() {
				time.Sleep(2 * time.Second)
				old.Close() //nolint:errcheck
			}()
		}
	}

	// reset endpoint
	return p.onSuccess(p.currentTransport)
}
