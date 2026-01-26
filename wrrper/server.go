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
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
	"wireflow/internal/config"
	internallog "wireflow/internal/log"
	"wireflow/pkg/wrrp"
)

type WRRPManager struct {
	mu      sync.Mutex
	streams map[uint64]*wrrp.Session
}

func (w *WRRPManager) Register(streamId uint64, stream wrrp.Stream) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.streams[streamId] = &wrrp.Session{
		ID:     streamId,
		Stream: stream,
		Type:   "WRRP",
	}

}

func (w *WRRPManager) Unregister(streamId uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.streams, streamId)
}

func (w *WRRPManager) Get(id uint64) *wrrp.Session {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.streams[id]
}

type Server struct {
	log         *internallog.Logger
	server      *http.Server
	wrrpManager *WRRPManager
}

func NewServer(flags *config.Flags) *Server {
	s := &Server{
		log: internallog.GetLogger("wrrp"),
		wrrpManager: &WRRPManager{
			streams: make(map[uint64]*wrrp.Session),
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/wrrp/v1/upgrade", s.wrrpUpgradeHandler)

	// 2. 配置 Server 实例
	httpServer := &http.Server{
		Addr:    flags.Listen,
		Handler: mux,
		// 注意：一旦 Hijack，这些超时限制将不再对该连接生效
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if flags.EnableTLS {
		httpServer.TLSConfig = &tls.Config{
			NextProtos: []string{"http/1.1"}, // 禁用 h2，确保 Hijack 可用
		}
	}

	httpServer.ErrorLog = log.New(os.Stderr, "HTTP Server Error: ", log.LstdFlags)
	s.server = httpServer
	return s
}

func (s *Server) Start() error {
	s.log.Info("WRRP Server (wrrper) is running", "listen", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

func (s *Server) wrrpUpgradeHandler(w http.ResponseWriter, r *http.Request) {

	// 1. 检查是否是升级请求
	if r.Header.Get("Upgrade") != "wrrp" {
		http.Error(w, "Expected WRRP Upgrade", http.StatusBadRequest)
		return
	}

	// 2. 准备接管连接
	rc := http.NewResponseController(w)

	// 在 Hijack 之前，你可以先写入 HTTP 101 响应
	w.Header().Set("Upgrade", "wrrp")
	w.Header().Set("Connection", "Upgrade")
	w.WriteHeader(http.StatusSwitchingProtocols)

	// 3. 执行 Hijack
	conn, bufrw, err := rc.Hijack()
	if err != nil {
		return // 此时不能用 http.Error，因为连接可能已损坏
	}

	// 在 Hijack 后的 conn 上设置
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetWriteBuffer(64 * 1024) // 64KB
		tcpConn.SetReadBuffer(64 * 1024)
		tcpConn.SetNoDelay(true) // 降低延迟的关键
	}

	// 4. 将连接交给 WRRP 处理器
	s.handleWRRPSession(conn, bufrw)
}

// handleWRRPSession handle core logic of WRRP session
func (s *Server) handleWRRPSession(conn net.Conn, bufrw *bufio.ReadWriter) {
	// 1. 包装连接，确保 Read/Write 走 bufrw 逻辑
	stream := &ReadWriterConn{Conn: conn, ReadWriter: bufrw}
	defer stream.Close()

	// 2. 设置读取超时（防止握手阶段无限期阻塞）
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// 3. 读取第一个 WRRP Header (必须是 Register)
	headBuf := make([]byte, wrrp.HeaderSize)
	if _, err := io.ReadFull(stream, headBuf); err != nil {
		return
	}

	header, err := wrrp.Unmarshal(headBuf)
	if err != nil || header.Cmd != wrrp.Register {
		// 非法协议或未注册先发数据，断开
		return
	}

	fromId := header.FromID
	// 4. 注册到全局管理器
	// 假设你有一个全局的 wrrpManager
	s.wrrpManager.Register(fromId, stream)
	defer s.wrrpManager.Unregister(fromId)

	// 5. 握手成功，重置超时（进入长连接模式）
	conn.SetReadDeadline(time.Time{})

	s.log.Info("[WRRP] New session registered", "fromId", header.FromID, "toId", header.ToID)

	// 6. 进入指令处理循环
	for {
		// 读取下一个 Header
		_, err = io.ReadFull(stream, headBuf)
		if err != nil {
			break // 客户端断开
		}

		h, err := wrrp.Unmarshal(headBuf)
		if err != nil {
			s.log.Error("invalid wrrp header", err)
			continue
		}

		switch h.Cmd {
		case wrrp.Ping:
			// 收到 Ping，什么都不用做，上面的 SetReadDeadline 已经完成了“续租”
			// 如果你想让客户端计算 RTT，也可以回发一个 CmdPong
			s.log.Debug("[WRRP] Receive Ping", "fromId", fromId)
			keepaliveAck(s.wrrpManager.Get(fromId))
			continue

		case wrrp.Forward, wrrp.Probe:
			// 处理转发逻辑
			targetID := h.ToID

			target := s.wrrpManager.Get(targetID)
			if target == nil {
				s.log.Warn("[WRRP] Target not found", "targetId", targetID)
				io.CopyN(io.Discard, stream, int64(h.PayloadLen)) //
				continue
			}
			// send header
			_, err = target.Stream.Write(headBuf)
			if err != nil {
				s.log.Error("relay packet failed", err)
				io.CopyN(io.Discard, stream, int64(h.PayloadLen))
				continue
			}
			_, err = io.CopyN(target.Stream, stream, int64(h.PayloadLen))
			if err != nil {
				s.log.Error("relay packet failed", err)
				if err = target.Stream.Close(); err != nil {
					s.log.Error("close target stream failed", err)
				}
			}

			s.log.Debug("[WRRP] Forward successfully", "fromId", fromId, "toId", targetID, "payloadLen", h.PayloadLen)
		}
	}
}

func keepaliveAck(dst *wrrp.Session) {
}
