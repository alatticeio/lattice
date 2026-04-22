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

package wrrper

import (
	"context"
	"crypto/tls"
	"net"
	"time"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/pkg/wrrp"

	"github.com/quic-go/quic-go"
	wgconn "golang.zx2c4.com/wireguard/conn"
	"google.golang.org/protobuf/proto"
)

var _ infra.Wrrp = (*QUICWRRPClient)(nil)

// QUICWRRPClient implements infra.Wrrp using QUIC datagrams for Forward/Probe
// and a QUIC control stream for registration. App-level keepalive is not needed
// because quic.Config.KeepAlivePeriod handles connection liveness.
type QUICWRRPClient struct {
	ctx    context.Context
	cancel context.CancelFunc

	log       *log.Logger
	localId   infra.PeerID
	serverURL string
	conn      *quic.Conn
	control   *quic.Stream

	onMessage func(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error
	probeChan chan *Task
}

// NewQUICWrrpClient creates a QUIC WRRP client, connects, and registers.
func NewQUICWrrpClient(
	ctx context.Context,
	localID infra.PeerID,
	url string,
	onMessage func(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error,
) (*QUICWRRPClient, error) {
	ctx, cancel := context.WithCancel(ctx)
	c := &QUICWRRPClient{
		ctx:       ctx,
		cancel:    cancel,
		log:       log.GetLogger("wrrper-quic"),
		localId:   localID,
		serverURL: url,
		probeChan: make(chan *Task, 1024),
		onMessage: onMessage,
	}

	go c.probeWorker()

	if err := c.Connect(); err != nil {
		cancel()
		return nil, err
	}

	return c, nil
}

// Connect dials the QUIC server, opens the control stream, and registers.
func (c *QUICWRRPClient) Connect() error {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
		NextProtos:         []string{"wrrp"},
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

	return c.register()
}

// Close cancels the client context and closes the underlying QUIC connection,
// which unblocks ReceiveFunc and causes all probeWorker goroutines to exit.
func (c *QUICWRRPClient) Close() error {
	c.cancel()
	if c.conn != nil {
		return c.conn.CloseWithError(0, "closed")
	}
	return nil
}

func (c *QUICWRRPClient) register() error {
	header := &wrrp.Header{
		Magic:      wrrp.MagicNumber,
		Version:    1,
		Cmd:        wrrp.Register,
		PayloadLen: 0,
		FromID:     c.localId.ToUint64(),
	}
	_, err := c.control.Write(header.Marshal())
	return err
}

// RemoteAddr returns the remote address of the QUIC connection.
func (c *QUICWRRPClient) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Send transmits a WRRP frame (header + data) as a QUIC datagram.
func (c *QUICWRRPClient) Send(ctx context.Context, targetId uint64, wrrpType uint8, data []byte) error {
	header := &wrrp.Header{
		Magic:      wrrp.MagicNumber,
		Version:    1,
		Cmd:        wrrpType,
		PayloadLen: uint32(len(data)),
		FromID:     c.localId.ToUint64(),
		ToID:       targetId,
	}
	frame := append(header.Marshal(), data...) //nolint:gocritic
	return c.conn.SendDatagram(frame)
}

// ReceiveFunc returns a WireGuard ReceiveFunc that reads incoming QUIC datagrams.
func (c *QUICWRRPClient) ReceiveFunc() wgconn.ReceiveFunc {
	return func(packets [][]byte, sizes []int, eps []wgconn.Endpoint) (n int, err error) {
		for {
			data, recvErr := c.conn.ReceiveDatagram(c.ctx)
			if recvErr != nil {
				c.log.Error("QUIC datagram receive error", recvErr)
				return 0, recvErr
			}

			if len(data) < wrrp.HeaderSize {
				c.log.Warn("datagram too short", "len", len(data))
				continue
			}

			header, parseErr := wrrp.Unmarshal(data[:wrrp.HeaderSize])
			if parseErr != nil {
				c.log.Error("failed to parse WRRP header", parseErr)
				continue
			}

			c.log.Debug("recv datagram", "cmd", header.Cmd, "bytes", header.PayloadLen)

			switch header.Cmd {
			case wrrp.Forward:
				payload := data[wrrp.HeaderSize:]
				copy(packets[0], payload)
				sizes[0] = len(payload)
				eps[0] = &infra.WRRPEndpoint{
					Addr:          infra.WrrpFakeAddrPort(header.FromID),
					RemoteId:      header.FromID,
					TransportType: infra.WRRP,
				}
				return 1, nil

			case wrrp.Probe:
				payload := data[wrrp.HeaderSize:]
				buf := make([]byte, len(payload))
				copy(buf, payload)
				select {
				case c.probeChan <- &Task{SessionID: header.FromID, Data: buf}:
				default:
					c.log.Warn("probe task dropped: channel at capacity")
				}
				// continue reading; return 0 to WireGuard
				continue

			default:
				c.log.Debug("unknown QUIC WRRP command, ignoring", "cmd", header.Cmd)
				continue
			}
		}
	}
}

func (c *QUICWRRPClient) probeWorker() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case task := <-c.probeChan:
			var packet grpc.SignalPacket
			if err := proto.Unmarshal(task.Data, &packet); err != nil {
				c.log.Error("failed to unmarshal probe packet", err)
				continue
			}
			if err := c.onMessage(c.ctx, infra.FromUint64(packet.SenderId), &packet); err != nil {
				c.log.Error("probe handler returned error", err)
			}
		}
	}
}
