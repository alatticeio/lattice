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
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/pkg/wrrp"

	"golang.zx2c4.com/wireguard/conn"
	"google.golang.org/protobuf/proto"
)

var (
	_ infra.Wrrp = (*WRRPClient)(nil)
)

const (
	sendChanDepth = 256
	writerBufSize = 128 * 1024
	probeChanSize = 1024
)

// WRRPClient multiplexes WireGuard traffic for multiple peers over a single
// persistent TCP connection to the WRRP relay server.
//
// Write-path design
//
// The naive approach of calling Conn.Write twice per frame (header then
// payload) has two problems:
//
//  1. Concurrent callers interleave bytes on the wire, corrupting the stream.
//  2. A full TCP send buffer blocks the calling goroutine (WireGuard's
//     encryption worker), which can cascade into device-level stalls.
//
// Instead, Send() pre-marshals header+payload into a single []byte and
// enqueues it on sendCh.  A dedicated writerLoop goroutine is the only
// goroutine that writes to Conn; it drains sendCh with a bufio.Writer so
// small frames are coalesced into fewer syscalls.  If sendCh is full the
// frame is dropped and Send returns an error (back-pressure).
type WRRPClient struct {
	ctx    context.Context
	cancel context.CancelFunc

	log       *log.Logger
	localId   infra.PeerID
	ServerURL string
	Conn      net.Conn
	Reader    *bufio.Reader

	onMessage func(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error

	probeChan chan *Task
	sendCh    chan []byte // pre-marshaled frames; drained by writerLoop
}

type Task struct {
	SessionID uint64
	Data      []byte
}

func (c *WRRPClient) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

func NewWrrpClient(ctx context.Context, localID infra.PeerID, url string, onMessage func(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error) (*WRRPClient, error) {
	ctx, cancel := context.WithCancel(ctx)
	c := &WRRPClient{
		ctx:       ctx,
		cancel:    cancel,
		log:       log.GetLogger("wrrper"),
		ServerURL: url,
		probeChan: make(chan *Task, probeChanSize),
		sendCh:    make(chan []byte, sendChanDepth),
		localId:   localID,
		onMessage: onMessage,
	}

	go c.probeWorker()

	if err := c.Connect(); err != nil {
		cancel()
		return nil, err
	}

	go c.writerLoop()

	return c, nil
}

// Close cancels the client context and closes the underlying TCP connection,
// which unblocks ReceiveFunc's io.ReadFull and causes all goroutines to exit.
func (c *WRRPClient) Close() error {
	c.cancel()
	if c.Conn != nil {
		return c.Conn.Close()
	}
	return nil
}

func (c *WRRPClient) probeWorker() {
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

// Connect establishes the TCP connection, performs the HTTP Upgrade handshake,
// and sends the WRRP Register frame.  It must complete before writerLoop
// starts so that register() can write directly without contention.
// nolint:all
func (c *WRRPClient) Connect() error {
	conn, err := net.Dial("tcp", c.ServerURL)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", "/wrrp/v1/upgrade", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Upgrade", "wrrp")
	req.Header.Set("Connection", "Upgrade")

	if err = req.Write(conn); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("upgrade failed: %v", err)
	}

	c.Conn = conn
	c.Reader = reader

	// Direct write is safe here: writerLoop has not started yet.
	return c.register()
}

func (c *WRRPClient) register() error {
	header := &wrrp.Header{
		Magic:      wrrp.MagicNumber,
		Version:    1,
		Cmd:        wrrp.Register,
		PayloadLen: 0,
		FromID:     c.localId.ToUint64(),
	}
	_, err := c.Conn.Write(header.Marshal())
	return err
}

// marshalFrame assembles header + payload into one contiguous []byte.
// Sending a single slice avoids the two-Write race and lets the OS write
// the entire frame atomically from the writer goroutine's perspective.
func marshalFrame(fromID, toID uint64, cmd uint8, payload []byte) []byte {
	h := &wrrp.Header{
		Magic:      wrrp.MagicNumber,
		Version:    1,
		Cmd:        cmd,
		PayloadLen: uint32(len(payload)),
		FromID:     fromID,
		ToID:       toID,
	}
	hb := h.Marshal()
	frame := make([]byte, len(hb)+len(payload))
	copy(frame, hb)
	copy(frame[len(hb):], payload)
	return frame
}

// Send enqueues a pre-marshaled frame for the writer goroutine.
// It never blocks on TCP; if sendCh is full the frame is dropped.
func (c *WRRPClient) Send(ctx context.Context, targetId uint64, wrrpType uint8, data []byte) error {
	frame := marshalFrame(c.localId.ToUint64(), targetId, wrrpType, data)
	select {
	case c.sendCh <- frame:
		return nil
	default:
		c.log.Warn("send channel full, dropping frame", "dst", targetId)
		return fmt.Errorf("wrrp: send channel full")
	}
}

// writerLoop is the sole goroutine that writes to c.Conn after Connect returns.
// Using a bufio.Writer coalesces small frames into fewer syscalls.
func (c *WRRPClient) writerLoop() {
	w := bufio.NewWriterSize(c.Conn, writerBufSize)
	for {
		// Block until the first frame or context cancellation.
		select {
		case <-c.ctx.Done():
			return
		case frame := <-c.sendCh:
			if _, err := w.Write(frame); err != nil {
				c.log.Error("TCP write failed", err)
				return
			}
		}

		// Drain any additional frames that arrived while we were writing,
		// then flush once — avoids a syscall per frame under load.
		drained := true
		for drained {
			select {
			case frame := <-c.sendCh:
				if _, err := w.Write(frame); err != nil {
					c.log.Error("TCP write failed", err)
					return
				}
			default:
				drained = false
			}
		}

		if err := w.Flush(); err != nil {
			c.log.Error("TCP flush failed", err)
			return
		}
	}
}

// ReceiveFunc using for Bind to handle data in wireguard
func (c *WRRPClient) ReceiveFunc() conn.ReceiveFunc {
	return func(packets [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		headBufp := wrrp.GetHeaderBuffer()
		defer wrrp.PutHeaderBuffer(headBufp)
		headBuf := *headBufp
		if _, err = io.ReadFull(c.Reader, headBuf); err != nil {
			c.log.Error("server connection lost", err)
			return
		}

		header, err := wrrp.Unmarshal(headBuf)
		if err != nil {
			c.log.Error("failed to parse WRRP header", err)
			return 0, err
		}
		c.log.Debug("recv", "cmd", header.Cmd, "bytes", header.PayloadLen)
		switch header.Cmd {
		case wrrp.Probe:
			bufp := (*wrrp.GetPayloadBuffer())[:header.PayloadLen]
			defer wrrp.PutPayloadBuffer(&bufp)
			if _, err = io.ReadFull(c.Reader, bufp); err != nil {
				c.log.Error("server connection lost", err)
				return 0, nil
			}
			select {
			case c.probeChan <- &Task{SessionID: header.FromID, Data: bufp}:
			default:
				c.log.Warn("probe task dropped: channel at capacity")
			}
			return 0, nil

		case wrrp.Forward:
			if _, err = io.ReadFull(c.Reader, packets[0][:header.PayloadLen]); err != nil {
				c.log.Error("server connection lost", err)
				return 0, err
			}
			sizes[0] = int(header.PayloadLen)
			eps[0] = &infra.WRRPEndpoint{
				Addr:          infra.WrrpFakeAddrPort(header.FromID),
				RemoteId:      header.FromID,
				TransportType: infra.WRRP,
			}
			return 1, nil

		default:
			payloadLen := int64(header.PayloadLen)
			if payloadLen > 0 {
				if _, err = io.CopyN(io.Discard, c.Reader, payloadLen); err != nil {
					c.log.Error("server connection lost", err)
					return 0, err
				}
				c.log.Warn("unknown WRRP command discarded", "cmd", header.Cmd)
			}
		}

		return 0, nil
	}
}
