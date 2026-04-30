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
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/grpc"

	"github.com/quic-go/quic-go"
	wgconn "golang.zx2c4.com/wireguard/conn"
)

var _ infra.Wrrp = (*QUICClient)(nil)

// QUICClient implements infra.Wrrp using QUIC datagrams for Forward/Probe
// and a QUIC control stream for registration. App-level keepalive is not needed
// because quic.Config.KeepAlivePeriod handles connection liveness.
type QUICClient struct {
	*lrpClient
	conn      *quic.Conn
	control   *quic.Stream
	ready     chan struct{}
	readyOnce sync.Once
}

// NewQUICClient creates a new QUIC LRP client, connects, and registers.
func NewQUICClient(ctx context.Context, localID infra.PeerID, url string, onMessage func(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error) (*QUICClient, error) {
	ctx, cancel := context.WithCancel(ctx)
	c := &QUICClient{
		lrpClient: &lrpClient{
			ctx:       ctx,
			cancel:    cancel,
			log:       log.GetLogger("lrp-quic"),
			localId:   localID,
			serverURL: url,
			probeCh:   make(chan *Task, probeChanSize),
			onMessage: onMessage,
		},
		ready: make(chan struct{}),
	}

	go c.probeWorker()

	if err := c.Connect(); err != nil {
		cancel()
		return nil, err
	}

	return c, nil
}

// Connect dials the QUIC server, opens the control stream, and registers.
func (c *QUICClient) Connect() error {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
		NextProtos:         []string{"lrp"},
	}
	quicCfg := &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  90 * time.Second,
		KeepAlivePeriod: 25 * time.Second,
	}

	conn, err := quic.DialAddr(c.ctx, c.serverURL, tlsCfg, quicCfg)
	if err != nil {
		return err
	}

	ctrl, err := conn.OpenStreamSync(c.ctx)
	if err != nil {
		conn.CloseWithError(0, "open stream failed") //nolint:errcheck
		return err
	}

	c.conn = conn
	c.control = ctrl

	if err = c.register(ctrl); err != nil {
		conn.CloseWithError(0, "register failed") //nolint:errcheck
		return err
	}

	c.readyOnce.Do(func() { close(c.ready) })
	return nil
}

// Close cancels the client context and closes the underlying QUIC connection.
func (c *QUICClient) Close() error {
	c.cancel()
	if c.conn != nil {
		return c.conn.CloseWithError(0, "closed")
	}
	return nil
}

// RemoteAddr returns the remote address of the QUIC connection.
func (c *QUICClient) RemoteAddr() net.Addr {
	if c.conn != nil {
		return c.conn.RemoteAddr()
	}
	return nil
}

// Send transmits a LRP frame (header + data) as a QUIC datagram.
func (c *QUICClient) Send(ctx context.Context, targetId uint64, lrpType uint8, data []byte) error {
	frame := c.makeFrame(targetId, lrpType, data)
	return c.conn.SendDatagram(frame)
}

// ReceiveFunc returns a WireGuard ReceiveFunc that reads incoming QUIC datagrams.
func (c *QUICClient) ReceiveFunc() wgconn.ReceiveFunc {
	return func(packets [][]byte, sizes []int, eps []wgconn.Endpoint) (n int, err error) {
		for {
			data, recvErr := c.conn.ReceiveDatagram(c.ctx)
			if recvErr != nil {
				return 0, recvErr
			}

			if len(data) < HeaderSize {
				c.log.Warn("datagram too short", "len", len(data))
				continue
			}

			header, parseErr := Unmarshal(data[:HeaderSize])
			if parseErr != nil {
				c.log.Error("failed to parse LRP header", parseErr)
				continue
			}

			switch header.Cmd {
			case Forward:
				payload := data[HeaderSize:]
				if len(payload) > len(packets[0]) {
					c.log.Warn("forward payload exceeds buffer", "need", len(payload), "have", len(packets[0]))
					continue
				}
				copy(packets[0], payload)
				sizes[0] = len(payload)
				eps[0] = &infra.WRRPEndpoint{
					Addr:          infra.WrrpFakeAddrPort(uint64(header.ToID)),
					RemoteId:      uint64(header.ToID),
					TransportType: infra.WRRP,
				}
				return 1, nil

			case Probe:
				payload := data[HeaderSize:]
				buf := make([]byte, len(payload))
				copy(buf, payload)
				select {
				case c.probeCh <- &Task{SessionID: uint64(header.ToID), Data: buf}:
				default:
					c.log.Warn("probe task dropped: channel at capacity")
				}
				continue

			default:
				c.log.Debug("unknown LRP command, ignoring", "cmd", header.Cmd)
				continue
			}
		}
	}
}
