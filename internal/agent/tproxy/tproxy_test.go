//go:build linux

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

package tproxy_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/agent/tproxy"
)

// TestProxyDialAndCopy verifies that the Proxy bridges data between
// two net.Conn pairs. We skip SO_ORIGINAL_DST here (requires real
// iptables redirect) and test only the copy/dial logic using a
// direct DialFunc that returns a preconfigured conn.
func TestProxyDialAndCopy(t *testing.T) {
	// Create a pair of in-memory connections to act as "remote".
	remoteA, remoteB := net.Pipe()
	defer remoteA.Close()
	defer remoteB.Close()

	// Dial function returns remoteA for any address.
	dialFn := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return remoteA, nil
	}

	proxy := &tproxy.Proxy{
		Addr: "127.0.0.1:0",
		Dial: dialFn,
		OrigDst: func(_ *net.TCPConn) (string, error) {
			return "1.2.3.4:80", nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := proxy.Start(ctx); err != nil {
		t.Fatalf("proxy.Start: %v", err)
	}

	// Connect to proxy.
	conn, err := net.Dial("tcp", proxy.ListenAddr())
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()

	// Send data through proxy → remoteA → remoteB reads it.
	go func() {
		conn.Write([]byte("hello")) //nolint:errcheck
		conn.Close()
	}()

	buf := make([]byte, 5)
	remoteB.SetDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	n, err := io.ReadFull(remoteB, buf)
	if err != nil {
		t.Fatalf("read from remoteB: %v (n=%d)", err, n)
	}
	if string(buf) != "hello" {
		t.Errorf("expected 'hello', got %q", buf)
	}
}
