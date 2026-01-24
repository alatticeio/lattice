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
	"sync"
	"time"
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

type WRRPClient struct {
	log       *log.Logger
	mu        sync.Mutex
	localId   infra.PeerID
	ServerURL string
	Conn      net.Conn
	Reader    *bufio.Reader

	// call back for wrrp probe
	onMessage func(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error

	probeChan chan *Task
}

func (c *WRRPClient) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

type Task struct {
	SessionID uint64
	Data      []byte
}

func NewWrrpClient(localID infra.PeerID, url string) (*WRRPClient, error) {

	c := &WRRPClient{
		log:       log.GetLogger("wrrper"),
		ServerURL: url,
		probeChan: make(chan *Task, 1024),
		localId:   localID,
	}

	for i := 0; i < 3; i++ {
		go c.probeWorker()
	}

	return c, nil
}

func (c *WRRPClient) probeWorker() {
	for task := range c.probeChan {
		var packet grpc.SignalPacket
		if err := proto.Unmarshal(task.Data, &packet); err != nil {
			c.log.Error("invalid packet", err)
			continue
		}

		if err := c.onMessage(context.Background(), infra.FromUint64(packet.SenderId), &packet); err != nil {
			c.log.Error("handle probe failed", err)
		}
	}
}

type ClientOption func(*WRRPClient)

func WithOnMessage(fn func(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error) ClientOption {
	return func(c *WRRPClient) {
		c.onMessage = fn
	}
}

func (c *WRRPClient) Configure(opts ...ClientOption) {
	for _, opt := range opts {
		opt(c)
	}
}

func (c *WRRPClient) Connect() error {
	// 1. 建立 TCP 连接 (如果是 https 则需要 tls.Dial)
	conn, err := net.Dial("tcp", c.ServerURL)
	if err != nil {
		return err
	}

	// 2. 手动构造 HTTP Upgrade 请求
	// 注意：不能直接用 http.Get，因为我们需要拿回底层的 conn
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

	// 4. 接管连接
	c.Conn = conn
	c.Reader = reader

	// 5. 立即发送 WRRP 注册报文 (Register)
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

// Send 向指定的目标 Peer 发送数据
func (c *WRRPClient) Send(ctx context.Context, targetId uint64, wrrpType uint8, data []byte) error {
	header := &wrrp.Header{
		Magic:      wrrp.MagicNumber,
		Version:    1,
		Cmd:        wrrpType,
		PayloadLen: uint32(len(data)),
		FromID:     c.localId.ToUint64(),
		ToID:       targetId,
	}

	// 发送 Header + Payload
	if _, err := c.Conn.Write(header.Marshal()); err != nil {
		return err
	}
	_, err := c.Conn.Write(data)
	return err
}

// ReceiveFunc using for Bind to handle data in wireguard
func (c *WRRPClient) ReceiveFunc() conn.ReceiveFunc {
	return func(packets [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		headBufp := wrrp.GetHeaderBuffer()
		defer wrrp.PutHeaderBuffer(headBufp)
		headBuf := *headBufp
		if _, err = io.ReadFull(c.Reader, headBuf); err != nil {
			c.log.Error("Connection closed by server", err)
			return
		}

		header, err := wrrp.Unmarshal(headBuf)
		if err != nil {
			c.log.Error("invalid wrrp header", err)
			return 0, err
		}
		c.log.Info("Receiving from wrrp server", "type", header.Cmd, "payloadLen", header.PayloadLen)
		switch header.Cmd {
		case wrrp.Probe:
			// 1. 读取 Probe 数据到临时缓冲区（不要占用 WireGuard 的 packets[0]）
			bufp := (*wrrp.GetPayloadBuffer())[:header.PayloadLen]
			defer wrrp.PutPayloadBuffer(&bufp)
			_, err = io.ReadFull(c.Reader, bufp)
			if err != nil {
				c.log.Error("Connection closed by server", err)
				return 0, nil
			}

			select {
			case c.probeChan <- &Task{SessionID: header.FromID, Data: bufp}:
			default:
				c.log.Warn("Probe channel is full, dropped probe task")
			}
			return 0, nil
		case wrrp.Forward:
			if _, err = io.ReadFull(c.Reader, packets[0][:header.PayloadLen]); err != nil {
				c.log.Error("Connection closed by server", err)
				return 0, err
			}

			eps[0] = &infra.WRRPEndpoint{
				RemoteId: header.FromID,
			}

			return int(header.PayloadLen), nil
		default:
			// must discard unknown command
			payloadLen := int64(header.PayloadLen)
			if payloadLen > 0 {
				_, err = io.CopyN(io.Discard, c.Reader, payloadLen)
				if err != nil {
					c.log.Error("Connection closed by server", err)
					return 0, err
				}

				c.log.Warn("unknow wrrp command discarded", "cmd", header.Cmd)
			}
		}

		return 0, nil
	}
}

func (c *WRRPClient) startKeepAlive(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 构造一个 Ping 包
			header := &wrrp.Header{
				Magic:      wrrp.MagicNumber,
				Version:    1,
				Cmd:        wrrp.Ping,
				PayloadLen: 0,
				FromID:     c.localId.ToUint64(),
			}

			c.mu.Lock() // 建议给 Conn 加锁，防止与数据发送冲突
			_, err := c.Conn.Write(header.Marshal())
			c.mu.Unlock()

			if err != nil {
				fmt.Printf("[WRRP] KeepAlive failed: %v\n", err)
				return
			}
		}
	}
}
