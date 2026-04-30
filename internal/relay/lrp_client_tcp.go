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
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/grpc"

	wgconn "golang.zx2c4.com/wireguard/conn"
)

var _ infra.Wrrp = (*TCPClient)(nil)

// TCPClient implements infra.Wrrp using a persistent TCP connection with
// HTTP upgrade handshake, buffered writer, and keepalive loop.
type TCPClient struct {
	*lrpClient
	mu        sync.Mutex
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	sendCh    chan []byte
	ready     chan struct{}
	readyOnce sync.Once
}

// NewTCPClient creates a new TCP LRP client, connects, and registers.
func NewTCPClient(ctx context.Context, localID infra.PeerID, url string, onMessage func(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error) (*TCPClient, error) {
	ctx, cancel := context.WithCancel(ctx)
	c := &TCPClient{
		lrpClient: &lrpClient{
			ctx:       ctx,
			cancel:    cancel,
			log:       log.GetLogger("lrp-tcp"),
			localId:   localID,
			serverURL: url,
			probeCh:   make(chan *Task, probeChanSize),
			onMessage: onMessage,
		},
		sendCh: make(chan []byte, sendChanDepth),
		ready:  make(chan struct{}),
	}

	go c.probeWorker()

	if err := c.Connect(); err != nil {
		cancel()
		return nil, err
	}

	go c.writerLoop()
	go c.keepaliveLoop()

	return c, nil
}

// Connect establishes the TCP connection, performs the HTTP Upgrade handshake,
// and sends the LRP Register frame.
func (c *TCPClient) Connect() error {
	conn, err := net.Dial("tcp", c.serverURL)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", "/lrp/v1/upgrade", nil)
	if err != nil {
		conn.Close()
		return err
	}
	req.Header.Set("Upgrade", "lrp")
	req.Header.Set("Connection", "Upgrade")

	if err = req.Write(conn); err != nil {
		conn.Close()
		return err
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req) //nolint:bodyclose // resp.Body wraps conn; we take ownership of the raw connection
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return fmt.Errorf("upgrade failed: %v", err)
	}
	// resp.Body wraps the underlying conn reader. We discard the upgrade response
	// and take ownership of the raw connection, so we must NOT close resp.Body.

	c.mu.Lock()
	c.conn = conn
	c.reader = reader
	c.writer = bufio.NewWriterSize(conn, writerBufSize)
	c.mu.Unlock()

	if err = c.register(c); err != nil {
		conn.Close()
		return err
	}

	c.readyOnce.Do(func() { close(c.ready) })
	return nil
}

// Close cancels the client context and closes the underlying TCP connection.
func (c *TCPClient) Close() error {
	c.cancel()
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

// RemoteAddr returns the remote address of the TCP connection.
func (c *TCPClient) RemoteAddr() net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.RemoteAddr()
	}
	return nil
}

// Send enqueues a pre-marshaled LRP frame for the writer goroutine.
func (c *TCPClient) Send(ctx context.Context, targetId uint64, lrpType uint8, data []byte) error {
	frame := c.makeFrame(targetId, lrpType, data)
	select {
	case c.sendCh <- frame:
		return nil
	default:
		c.log.Warn("send channel full, dropping frame", "dst", targetId)
		return fmt.Errorf("lrp: send channel full")
	}
}

// writerLoop is the sole goroutine that writes to the TCP connection.
func (c *TCPClient) writerLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case frame := <-c.sendCh:
			c.mu.Lock()
			w := c.writer
			c.mu.Unlock()
			if w == nil {
				return
			}
			if _, err := w.Write(frame); err != nil {
				c.log.Error("TCP write failed", err)
				return
			}
		}

		// Batch drain: grab any additional frames ready to coalesce writes.
		c.mu.Lock()
		w := c.writer
		c.mu.Unlock()
		if w == nil {
			return
		}
		batchSize := len(c.sendCh)
		for i := 0; i < batchSize; i++ {
			frame := <-c.sendCh
			if _, err := w.Write(frame); err != nil {
				c.log.Error("TCP write failed", err)
				return
			}
		}

		if err := w.Flush(); err != nil {
			c.log.Error("TCP flush failed", err)
			return
		}
	}
}

// keepaliveLoop sends KeepAlive frames every 20 seconds.
func (c *TCPClient) keepaliveLoop() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.Send(c.ctx, 0, KeepAlive, nil) //nolint:errcheck
		}
	}
}

// ReceiveFunc returns a WireGuard ReceiveFunc that reads incoming LRP frames.
func (c *TCPClient) ReceiveFunc() wgconn.ReceiveFunc {
	return func(packets [][]byte, sizes []int, eps []wgconn.Endpoint) (n int, err error) {
		c.mu.Lock()
		conn := c.conn
		reader := c.reader
		c.mu.Unlock()

		if conn == nil || reader == nil {
			return 0, fmt.Errorf("lrp: not connected")
		}

		// 30s read deadline — prevents permanent WireGuard read block.
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		headBufp := GetHeaderBuffer()
		defer PutHeaderBuffer(headBufp)
		headBuf := *headBufp

		if _, err = io.ReadFull(reader, headBuf); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return 0, nil // timeout, not a real error
			}
			return 0, err
		}

		header, err := Unmarshal(headBuf)
		if err != nil {
			c.log.Error("failed to parse LRP header", err)
			return 0, err
		}

		switch header.Cmd {
		case Probe:
			if header.PayloadLen > MaxProbePayload {
				c.log.Warn("probe payload too large", "bytes", header.PayloadLen)
				return 0, nil
			}
			buf := make([]byte, header.PayloadLen)
			if _, err = io.ReadFull(reader, buf); err != nil {
				return 0, err
			}
			select {
			case c.probeCh <- &Task{SessionID: uint64(header.ToID), Data: buf}:
			default:
				c.log.Warn("probe task dropped: channel at capacity")
			}
			return 0, nil

		case Forward:
			if int(header.PayloadLen) > len(packets[0]) {
				c.log.Warn("forward payload exceeds buffer", "need", header.PayloadLen, "have", len(packets[0]))
				return 0, nil
			}
			if _, err = io.ReadFull(reader, packets[0][:header.PayloadLen]); err != nil {
				return 0, err
			}
			sizes[0] = int(header.PayloadLen)
			eps[0] = &infra.WRRPEndpoint{
				Addr:          infra.WrrpFakeAddrPort(uint64(header.ToID)),
				RemoteId:      uint64(header.ToID),
				TransportType: infra.WRRP,
			}
			return 1, nil

		default:
			if header.PayloadLen > 0 {
				_, _ = io.CopyN(io.Discard, reader, int64(header.PayloadLen))
			}
			c.log.Warn("unknown LRP command discarded", "cmd", header.Cmd)
			return 0, nil
		}
	}
}

// Write satisfies the writer interface for register().
func (c *TCPClient) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return 0, fmt.Errorf("lrp: not connected")
	}
	return c.conn.Write(p)
}
