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

// Package tproxy implements a transparent TCP proxy that recovers the original
// destination of iptables REDIRECT'd connections via SO_ORIGINAL_DST and
// forwards them through a caller-supplied dial function.
package tproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Proxy is a transparent TCP proxy. It listens on Addr, recovers the original
// destination of each accepted connection via SO_ORIGINAL_DST (set by iptables
// REDIRECT), then dials it via Dial and copies data bidirectionally.
type Proxy struct {
	// Addr is the TCP address to listen on (e.g. "0.0.0.0:15001").
	Addr string

	// Dial is called with the original destination address for each accepted
	// connection. Typically this dials through the gVisor netstack.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)

	// OrigDst overrides the SO_ORIGINAL_DST lookup. If nil, the real
	// getsockopt is used. Useful for testing without iptables.
	OrigDst func(conn *net.TCPConn) (string, error)

	ln net.Listener
}

// Start begins listening and handling connections. It returns once the listener
// is bound; connections are handled in background goroutines. The listener is
// closed when ctx is cancelled.
func (p *Proxy) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", p.Addr)
	if err != nil {
		return fmt.Errorf("tproxy: listen %s: %w", p.Addr, err)
	}
	p.ln = ln
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	go p.serve(ctx, ln)
	return nil
}

// ListenAddr returns the address the proxy is actually listening on.
// Call after Start.
func (p *Proxy) ListenAddr() string {
	if p.ln == nil {
		return ""
	}
	return p.ln.Addr().String()
}

func (p *Proxy) serve(ctx context.Context, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go p.handle(ctx, conn.(*net.TCPConn))
	}
}

func (p *Proxy) handle(ctx context.Context, src *net.TCPConn) {
	defer src.Close()

	origDstFn := p.OrigDst
	if origDstFn == nil {
		origDstFn = originalDst
	}
	dst, err := origDstFn(src)
	if err != nil {
		return
	}

	remote, err := p.Dial(ctx, "tcp", dst)
	if err != nil {
		return
	}
	defer remote.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(remote, src); done <- struct{}{} }() //nolint:errcheck
	go func() { io.Copy(src, remote); done <- struct{}{} }() //nolint:errcheck
	<-done
}

// originalDst reads the original destination address of an iptables REDIRECT'd
// TCP connection using SO_ORIGINAL_DST getsockopt.
func originalDst(conn *net.TCPConn) (string, error) {
	f, err := conn.File()
	if err != nil {
		return "", fmt.Errorf("get file: %w", err)
	}
	defer f.Close()

	// sockaddr_in layout: sa_family(2) + port(2) + addr(4) + pad(8) = 16 bytes
	var raw [16]byte
	size := uint32(len(raw))
	_, _, errno := unix.Syscall6(
		unix.SYS_GETSOCKOPT,
		f.Fd(),
		unix.IPPROTO_IP,
		80, // SO_ORIGINAL_DST = 80
		uintptr(unsafe.Pointer(&raw[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if errno != 0 {
		return "", fmt.Errorf("getsockopt SO_ORIGINAL_DST: %w", errno)
	}
	port := int(raw[2])<<8 | int(raw[3])
	ip := net.IP(raw[4:8])
	return fmt.Sprintf("%s:%d", ip.String(), port), nil
}
