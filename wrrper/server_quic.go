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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"time"

	internallog "wireflow/internal/log"
	"wireflow/pkg/wrrp"

	quic "github.com/quic-go/quic-go"
)

// quicControlStream wraps a *quic.Stream and its parent *quic.Conn to implement wrrp.Stream.
type quicControlStream struct {
	stream *quic.Stream
	conn   *quic.Conn
}

func (s *quicControlStream) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s *quicControlStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *quicControlStream) Close() error {
	return s.conn.CloseWithError(0, "closed")
}

func (s *quicControlStream) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// QUICServer accepts QUIC connections and multiplexes WRRP sessions.
type QUICServer struct {
	log         *internallog.Logger
	wrrpManager *WRRPManager
}

// NewQUICServer creates a new QUICServer backed by the given WRRPManager.
func NewQUICServer(manager *WRRPManager) *QUICServer {
	return &QUICServer{
		log:         internallog.GetLogger("wrrp-quic"),
		wrrpManager: manager,
	}
}

// Start listens for QUIC connections on addr using the provided TLS config.
func (s *QUICServer) Start(addr string, tlsCfg *tls.Config) error {
	quicCfg := &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  90 * time.Second,
		KeepAlivePeriod: 25 * time.Second,
	}

	ln, err := quic.ListenAddr(addr, tlsCfg, quicCfg)
	if err != nil {
		return err
	}
	s.log.Info("QUIC WRRP relay server listening", "addr", addr)

	for {
		conn, err := ln.Accept(context.Background())
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *QUICServer) handleConn(conn *quic.Conn) {
	defer conn.CloseWithError(0, "session ended") //nolint:errcheck

	// Accept the control stream (first stream from the client).
	ctrl, err := conn.AcceptStream(context.Background())
	if err != nil {
		s.log.Error("failed to accept control stream", err)
		return
	}

	// Read the Register header.
	headBuf := make([]byte, wrrp.HeaderSize)
	if _, err = io.ReadFull(ctrl, headBuf); err != nil {
		s.log.Error("failed to read Register header", err)
		return
	}

	h, err := wrrp.Unmarshal(headBuf)
	if err != nil || h.Cmd != wrrp.Register {
		s.log.Warn("expected Register command")
		return
	}

	fromId := h.FromID
	ctrlStream := &quicControlStream{stream: ctrl, conn: conn}
	s.wrrpManager.RegisterQUIC(fromId, ctrlStream, conn)
	defer s.wrrpManager.Unregister(fromId)

	s.log.Info("QUIC session registered", "from", fromId)

	go s.relayDatagrams(conn, fromId)
	s.handleControlStream(ctrl, fromId)
}

func (s *QUICServer) relayDatagrams(conn *quic.Conn, fromId uint64) {
	for {
		data, err := conn.ReceiveDatagram(context.Background())
		if err != nil {
			s.log.Debug("datagram receive ended", "from", fromId, "err", err)
			return
		}

		if len(data) < wrrp.HeaderSize {
			s.log.Warn("datagram too short", "from", fromId, "len", len(data))
			continue
		}

		h, err := wrrp.Unmarshal(data[:wrrp.HeaderSize])
		if err != nil {
			s.log.Warn("invalid datagram header", "from", fromId, "err", err)
			continue
		}

		if h.Cmd != wrrp.Forward && h.Cmd != wrrp.Probe {
			s.log.Debug("ignoring non-data datagram", "cmd", h.Cmd)
			continue
		}

		if relayErr := s.wrrpManager.Relay(h.ToID, data); relayErr != nil {
			s.log.Warn("datagram relay failed", "from", fromId, "to", h.ToID, "err", relayErr)
		} else {
			s.log.Debug("datagram relayed", "from", fromId, "to", h.ToID)
		}
	}
}

func (s *QUICServer) handleControlStream(ctrl *quic.Stream, fromId uint64) {
	headBuf := make([]byte, wrrp.HeaderSize)
	for {
		_, err := io.ReadFull(ctrl, headBuf)
		if err != nil {
			s.log.Debug("control stream closed", "from", fromId)
			return
		}

		h, err := wrrp.Unmarshal(headBuf)
		if err != nil {
			s.log.Warn("invalid control header", "from", fromId, "err", err)
			return
		}

		if h.Cmd == wrrp.Ping {
			s.log.Debug("ping received on control stream", "from", fromId)
		}
	}
}

// GenerateSelfSignedTLS generates a self-signed RSA 2048 TLS certificate valid for 10 years.
func GenerateSelfSignedTLS() (*tls.Config, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"wrrp"},
	}, nil
}
