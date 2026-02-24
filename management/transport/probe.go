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
	localId   infra.PeerID
	remoteId  infra.PeerID
	iceDialer infra.Dialer
	state     ice.ConnectionState
	signal    infra.SignalService
	log       *log.Logger
	rtt       time.Duration // nolint

	started atomic.Bool

	// Add wrrp
	wrrpDialer infra.Dialer

	onSuccess        func(transport infra.Transport) error
	onFailure        func(error) error
	currentTransport infra.Transport
}

func (p *Probe) Handle(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error {
	switch packet.Dialer {
	case grpc.DialerType_ICE:
		return p.iceDialer.Handle(ctx, p.remoteId, packet)
	case grpc.DialerType_WRRP:
		return p.wrrpDialer.Handle(ctx, p.remoteId, packet)
	}

	return nil
}

func (p *Probe) OnConnectionStateChange(state ice.ConnectionState) {
	p.updateState(state)
	p.log.Debug("Setting new connection status", "state", state)
}

func (p *Probe) Start(ctx context.Context, remoteId infra.PeerID) error {
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

		p.currentTransport = t
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

// discover 实现了“谁快用谁”的竞速逻辑
func (p *Probe) discover(ctx context.Context) (infra.Transport, error) {
	// 用于接收第一个成功的 Transport
	result := make(chan infra.Transport, 2)
	// 用于接收所有的错误，只有全部失败才报错
	errs := make(chan error, 2)

	go func() {
		p.log.Debug("Starting ice dialer", "remoteId", p.remoteId)
		if err := p.iceDialer.Prepare(ctx, p.remoteId); err != nil {
			errs <- err
			return
		}
		t, err := p.iceDialer.Dial(ctx)
		if err != nil {
			errs <- err
			return
		}
		result <- t
		if err = p.handleUpgradeTransport(t); err != nil {
			errs <- err
			p.log.Error("Upgrade transport failed", err)
			return
		}
	}()

	// do not enable default
	if config.Conf.EnableWrrp {
		go func() {
			p.log.Debug("Starting wrrp dialer", "remoteId", p.remoteId)
			err := p.wrrpDialer.Prepare(ctx, p.remoteId)
			if err != nil {
				errs <- err
				return
			}
			// 内部包含：向中转服务器注册 -> 建立隧道
			t, err := p.wrrpDialer.Dial(ctx)
			if err != nil {
				errs <- err
				return
			}
			result <- t
		}()
	}

	// 3. 竞速决策逻辑
	var best infra.Transport
	select {
	case t := <-result:
		best = t
		// 特殊优化：如果 WRRP 先到了，我们可以额外等一小会儿（如 500ms）给 ICE 机会
		if t.Type() == infra.WRRP {
			select {
			case iceT := <-result:
				t.Close()
				best = iceT
			case <-time.After(500 * time.Millisecond):
			}
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return best, nil
}

func (p *Probe) handleUpgradeTransport(newTransport infra.Transport) error {
	p.log.Debug("Upgrade transport....", "newTransport", newTransport)
	p.mu.Lock()
	defer p.mu.Unlock()

	// 权重比较：直连优于中转
	if newTransport.Priority() > p.currentTransport.Priority() {
		old := p.currentTransport
		p.currentTransport = newTransport

		// 延迟关闭旧连接，确保缓冲区数据发完
		go func() {
			time.Sleep(2 * time.Second)
			old.Close()
		}()
	}

	// reset endpoint
	return p.onSuccess(p.currentTransport)
}
