//go:build !linux

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

// Package tproxy is a Linux-only transparent proxy; this stub satisfies the
// package on non-Linux platforms.
package tproxy

import (
	"context"
	"fmt"
	"net"
)

// Proxy is a no-op on non-Linux platforms.
type Proxy struct {
	Addr    string
	Dial    func(ctx context.Context, network, addr string) (net.Conn, error)
	OrigDst func(conn *net.TCPConn) (string, error)
}

// Start returns an error on non-Linux platforms.
func (p *Proxy) Start(_ context.Context) error {
	return fmt.Errorf("tproxy: transparent proxy is only supported on Linux")
}

// ListenAddr returns an empty string on non-Linux platforms.
func (p *Proxy) ListenAddr() string {
	return ""
}
