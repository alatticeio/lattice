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
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/alatticeio/lattice/internal/agent/config"
	internallog "github.com/alatticeio/lattice/internal/agent/log"
)

type Server struct {
	log        *internallog.Logger
	server     *http.Server
	sessionMgr *SessionManager
}

func NewServer(flags *config.Config) *Server {
	s := &Server{
		log:        internallog.GetLogger("bolt"),
		sessionMgr: NewSessionManager(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/bolt/v1/upgrade", s.boltUpgradeHandler)

	httpServer := &http.Server{
		Addr:         flags.Listen,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if flags.EnableTLS {
		httpServer.TLSConfig = &tls.Config{
			NextProtos: []string{"http/1.1"},
		}
	}

	httpServer.ErrorLog = log.New(os.Stderr, "HTTP Server Error: ", log.LstdFlags)
	s.server = httpServer
	return s
}

func (s *Server) Manager() *SessionManager {
	return s.sessionMgr
}

func (s *Server) Start() error {
	s.log.Info("LRP relay server listening", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) boltUpgradeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") != "bolt" {
		http.Error(w, "Expected Bolt Upgrade", http.StatusBadRequest)
		return
	}

	rc := http.NewResponseController(w)
	w.Header().Set("Upgrade", "bolt")
	w.Header().Set("Connection", "Upgrade")
	w.WriteHeader(http.StatusSwitchingProtocols)

	conn, bufrw, err := rc.Hijack()
	if err != nil {
		s.log.Error("failed to hijack connection", err)
		return
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetWriteBuffer(64 * 1024)
		_ = tcpConn.SetReadBuffer(64 * 1024)
		_ = tcpConn.SetNoDelay(true)
	}

	s.handleBoltSession(conn, bufrw)
}

func (s *Server) handleBoltSession(conn net.Conn, bufrw *bufio.ReadWriter) {
	stream := &ReadWriterConn{Conn: conn, ReadWriter: bufrw}
	defer stream.Close()

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	headBuf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(stream, headBuf); err != nil {
		s.log.Error("failed to read Register header", err)
		return
	}

	header, err := Unmarshal(headBuf)
	if err != nil || header.Cmd != Register {
		s.log.Warn("expected Register command", "err", err)
		return
	}

	fromId := uint64(header.ToID)
	s.sessionMgr.Register(fromId, &Session{
		ID:     fromId,
		Stream: stream,
		Type:   "TCP",
	})
	defer s.sessionMgr.Unregister(fromId)

	_ = conn.SetReadDeadline(time.Time{})
	s.log.Info("session registered", "from", fromId)

	for {
		_, err = io.ReadFull(stream, headBuf)
		if err != nil {
			break
		}

		h, err := Unmarshal(headBuf)
		if err != nil {
			s.log.Error("invalid lrp header", err)
			if h != nil && h.PayloadLen > 0 {
				_, _ = io.CopyN(io.Discard, stream, int64(h.PayloadLen))
			}
			continue
		}

		switch h.Cmd {
		case KeepAlive:
			_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			s.log.Debug("keepalive received", "from", fromId)

		case Forward, Probe:
			frame := make([]byte, HeaderSize+int(h.PayloadLen))
			copy(frame, headBuf)
			if h.PayloadLen > 0 {
				if _, err = io.ReadFull(stream, frame[HeaderSize:]); err != nil {
					s.log.Error("failed to read relay payload", err, "from", fromId, "to", h.ToID)
					continue
				}
			}

			if relayErr := s.sessionMgr.Relay(uint64(h.ToID), frame); relayErr != nil {
				s.log.Warn("relay failed", "from", fromId, "to", h.ToID, "err", relayErr)
			}
		}
	}
}
