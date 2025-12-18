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

package drp

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"time"
	"wireflow/internal/core/domain"
	drpgrpc "wireflow/internal/grpc"
	"wireflow/pkg/log"

	"golang.zx2c4.com/wireguard/conn"
)

// Proxy will send data to local engine when using drp relay,
// and also send data to other node from local engine
type Proxy struct {
	logger *log.Logger
	// Address is the address of the proxy server
	Addr  netip.AddrPort
	queue struct {
		drpClient     domain.DRPClient
		outBoundQueue chan *drpgrpc.DrpMessage
		inBoundQueue  chan *drpgrpc.DrpMessage
	}

	drpAddr      string
	offerHandler domain.OfferHandler
	manager      struct {
		msgManager   *MessageManager
		probeManager domain.ProberManager
	}

	proxyDo func(ctx context.Context, msg *drpgrpc.DrpMessage) error
}

type ProxyConfig struct {
	OfferHandler domain.OfferHandler
	DrpClient    domain.DRPClient
	DrpAddr      string
}

type ProxyOptions func(p *Proxy) error

func NewProxy(cfg *ProxyConfig) (*Proxy, error) {
	addr, err := net.ResolveTCPAddr("tcp", cfg.DrpAddr)
	if err != nil {
		return nil, err
	}
	addrPort := addr.AddrPort()
	proxy := &Proxy{
		logger:       log.NewLogger(log.Loglevel, "proxy"),
		offerHandler: cfg.OfferHandler,
		Addr:         addrPort, // Default address, can be set later
	}

	proxy.queue.outBoundQueue = make(chan *drpgrpc.DrpMessage, 10000)
	proxy.queue.inBoundQueue = make(chan *drpgrpc.DrpMessage, 10000)
	proxy.manager.msgManager = NewMessageManager()
	proxy.queue.drpClient = cfg.DrpClient
	return proxy, nil
}

func (p *Proxy) Start() error {
	return p.queue.drpClient.HandleMessage(context.Background(), p.queue.outBoundQueue, p.ReceiveMessage)
}

// ReceiveMessage receive message from drp server
func (p *Proxy) ReceiveMessage(ctx context.Context, msg *drpgrpc.DrpMessage) error {
	if msg.Body == nil {
		return errors.New("body is nil")
	}

	switch msg.MsgType {
	case drpgrpc.MessageType_MessageDrpDataType:
		// write data
		p.queue.inBoundQueue <- msg
	default:
		p.logger.Verbosef("receive from drp server type: %v, from: %v, to: %v", msg.MsgType, msg.From, msg.To)

		go func() {
			if err := p.offerHandler.ReceiveOffer(ctx, msg); err != nil {
				p.logger.Errorf("handle response failed: %v", err)
			}
		}()
	}

	return nil
}

func (p *Proxy) MakeReceiveFromDrp() conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		msg := <-p.queue.inBoundQueue
		// write to local engine
		switch msg.MsgType {
		case drpgrpc.MessageType_MessageDrpDataType:
			p.logger.Verbosef("client received drp data, time slapped: %v", time.Since(time.UnixMilli(msg.Timestamp)).Milliseconds())
			for i := 0; i < len(bufs); i++ {
				copy(bufs[i], msg.Body)
				sizes[i] = len(msg.Body)
				eps[i] = &domain.MagicEndpoint{
					Relay: &struct {
						FromType domain.EndpointType
						Status   bool
						From     string
						To       string
						Endpoint netip.AddrPort
					}{FromType: domain.DRP, Status: true, From: msg.To, To: msg.From, Endpoint: p.Addr},
				}
			}

			return len(bufs), nil
		default:
			p.logger.Errorf("unsupported message type: %v", msg.MsgType)
			return -1, errors.New("unsupported message type")
		}
	}

}

func (p *Proxy) Send(ep conn.Endpoint, bufs [][]byte) (err error) {
	var (
		from string
		to   string
	)
	if ep == nil {
		return errors.New("endpoint is nil")
	}

	if v, ok := ep.(domain.RelayEndpoint); ok {
		from = v.From()
		to = v.To()
	} else {
		return errors.New("unsupported endpoint type")
	}

	for i := 0; i < len(bufs); i++ {
		if bufs[i] == nil || len(bufs[i]) == 0 {
			return errors.New("buffer is nil or empty")
		}
		drpMesssage := p.GetMessageFromPool()
		drpMesssage.From = from
		drpMesssage.To = to
		drpMesssage.MsgType = drpgrpc.MessageType_MessageDrpDataType
		drpMesssage.Body = bufs[i]
		drpMesssage.Timestamp = time.Now().UnixMilli()

		p.queue.outBoundQueue <- drpMesssage
	}

	return nil
}

// WriteMessage will send actual message to data channel
func (p *Proxy) WriteMessage(ctx context.Context, msg *drpgrpc.DrpMessage) error {
	p.queue.outBoundQueue <- msg
	return nil
}
