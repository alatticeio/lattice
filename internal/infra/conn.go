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

package infra

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/pkg/wrrp"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	_ conn.Bind = (*DefaultBind)(nil)
)

// DefaultBind implements Bind for all platforms. While Windows has its own Bind
// (see bind_windows.go), it may fall back to DefaultBind.
// TODO: RemoveProbe usage of ipv{4,6}.PacketConn when net.UDPConn has comparable
// methods for sending and receiving multiple datagrams per-syscall. See the
// proposal in https://github.com/golang/go/issues/45886#issuecomment-1218301564.
type DefaultBind struct {
	logger       *log.Logger
	PublicKey    wgtypes.Key
	keyManager   KeyManager
	wrrperClient Wrrp

	// passThroughCh receives non-STUN packets forwarded by FilteringUDPMux (v4).
	// makeReceiveIPv4 reads from here instead of the raw socket.
	passThroughCh  <-chan PassThroughPacket
	// passThrough6Ch receives non-STUN packets forwarded by FilteringUDPMux (v6).
	// makeReceiveIPv6 reads from here instead of the raw socket.
	passThrough6Ch <-chan PassThroughPacket

	mu     sync.Mutex // protects all fields except as specified
	v4conn *net.UDPConn
	v6conn *net.UDPConn
	ipv4   *net.UDPConn
	ipv6   *net.UDPConn
	ipv4PC *ipv4.PacketConn // will be nil on non-Linux
	ipv6PC *ipv6.PacketConn // will be nil on non-Linux

	// these three fields are not guarded by mu
	udpAddrPool  sync.Pool
	ipv4MsgsPool sync.Pool
	ipv6MsgsPool sync.Pool

	blackhole4 bool
	blackhole6 bool
}

type BindConfig struct {
	Logger       *log.Logger
	V4Conn       *net.UDPConn
	V6Conn       *net.UDPConn
	PassThrough  <-chan PassThroughPacket // non-STUN v4 packets from FilteringUDPMux
	PassThrough6 <-chan PassThroughPacket // non-STUN v6 packets from FilteringUDPMux (v6)
	WrrpClient   Wrrp
	KeyManager   KeyManager
}

func NewBind(cfg *BindConfig) *DefaultBind {
	return &DefaultBind{
		logger:        cfg.Logger,
		v4conn:        cfg.V4Conn,
		v6conn:        cfg.V6Conn,
		passThroughCh:  cfg.PassThrough,
		passThrough6Ch: cfg.PassThrough6,
		keyManager:    cfg.KeyManager,
		wrrperClient:  cfg.WrrpClient,
		udpAddrPool: sync.Pool{
			New: func() any {
				return &net.UDPAddr{
					IP: make([]byte, 16),
				}
			},
		},

		ipv4MsgsPool: sync.Pool{
			New: func() any {
				msgs := make([]ipv4.Message, conn.IdealBatchSize)
				for i := range msgs {
					msgs[i].Buffers = make(net.Buffers, 1)
					msgs[i].OOB = make([]byte, srcControlSize)
				}
				return &msgs
			},
		},

		ipv6MsgsPool: sync.Pool{
			New: func() any {
				msgs := make([]ipv6.Message, conn.IdealBatchSize)
				for i := range msgs {
					msgs[i].Buffers = make(net.Buffers, 1)
					msgs[i].OOB = make([]byte, srcControlSize)
				}
				return &msgs
			},
		},
	}

}

func (b *DefaultBind) GetPackectConn4() net.PacketConn {
	return b.ipv4
}

func (b *DefaultBind) GetPackectConn6() net.PacketConn {
	return b.ipv6
}

func (b *DefaultBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	if strings.HasPrefix(s, "wrrp:") {
		_, after, ok := strings.Cut(s, "wrrp://")
		if !ok {
			return nil, errors.New("invalid wrrp endpoint")
		}
		remoteId, err := strconv.ParseUint(after, 10, 64)
		if err != nil {
			return nil, err
		}
		// Addr is not used for WRRP routing (Send routes via RemoteId through
		// wrrperClient), so we don't require a valid relay IP here.
		// We opportunistically fill it from the connected client for debugging.
		var addr netip.AddrPort
		if b.wrrperClient != nil {
			if ra := b.wrrperClient.RemoteAddr(); ra != nil {
				addr, _ = netip.ParseAddrPort(ra.String())
			}
		}
		return &WRRPEndpoint{
			Addr:          addr,
			RemoteId:      remoteId,
			TransportType: WRRP,
		}, nil
	}
	e, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}

	if IsWrrpFakeAddr(e.Addr()) {
		return &WRRPEndpoint{
			Addr:          e,
			RemoteId:      RemoteIdFromWrrpFakeAddr(e.Addr()),
			TransportType: WRRP,
		}, nil
	}

	return &WRRPEndpoint{
		Addr:          e,
		TransportType: ICE,
	}, nil
}

// listenNet will return udp and tcp conn on the same port.
func listenNet(network string, port int) (*net.UDPConn, int, error) {
	conn, err := listenConfig().ListenPacket(context.Background(), network, ":"+strconv.Itoa(port))
	if err != nil {
		return nil, 0, err
	}

	// Retrieve port.
	laddr := conn.LocalAddr()
	uaddr, err := net.ResolveUDPAddr(
		laddr.Network(),
		laddr.String(),
	)
	if err != nil {
		return nil, 0, err
	}
	return conn.(*net.UDPConn), uaddr.Port, nil
}

func ListenUDP(net string, uport uint16) (*net.UDPConn, int, error) {
	port := int(uport)
	conn, port, err := listenNet(net, port)
	if err != nil && !errors.Is(err, syscall.EAFNOSUPPORT) {
		return nil, 0, err
	}

	return conn, port, nil
}

// Open registers ReceiveFunc handlers for WireGuard.
//
// IPv4 receive: reads from passThroughCh, populated by FilteringUDPMux.readLoop
// (the sole reader of the v4 socket). STUN packets are routed to ICE; all others
// arrive here.
// IPv6 receive: reads from passThrough6Ch, populated by a separate
// FilteringUDPMux.readLoop for v6conn. When ICE over IPv6 is enabled, STUN
// packets on v6conn are also routed to the ICE mux, and WireGuard packets arrive
// here via the channel — eliminating the same race that existed on v4.
func (b *DefaultBind) Open(uport uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.ipv4 != nil || b.ipv6 != nil {
		return nil, 0, conn.ErrBindAlreadyOpen
	}

	port := int(uport)
	var fns []conn.ReceiveFunc

	// v4: FilteringUDPMux owns the socket; we read from the passthrough channel.
	if b.v4conn != nil {
		if runtime.GOOS == "linux" {
			b.ipv4PC = ipv4.NewPacketConn(b.v4conn)
		}
		fns = append(fns, b.makeReceiveIPv4())
		b.ipv4 = b.v4conn // kept so Send() knows v4 is open
	}

	// v6: FilteringUDPMux (v6) owns the socket; we read from passThrough6Ch.
	// ipv6PC is still set here for the send path (send6 uses WriteBatch on Linux).
	if b.v6conn != nil {
		if runtime.GOOS == "linux" {
			b.ipv6PC = ipv6.NewPacketConn(b.v6conn)
		}
		fns = append(fns, b.makeReceiveIPv6())
		b.ipv6 = b.v6conn
	}
	if len(fns) == 0 {
		return nil, 0, syscall.EAFNOSUPPORT
	}

	if config.Conf.EnableWrrp {
		fns = append(fns, b.wrrperClient.ReceiveFunc())
	}

	return fns, uint16(port), nil
}

// makeReceiveIPv4 returns a ReceiveFunc that reads WireGuard packets from
// passThroughCh. FilteringUDPMux.readLoop is the sole reader of the real
// socket and forwards non-STUN packets here, so there is no race with the
// mux's internal connWorker goroutine.
func (b *DefaultBind) makeReceiveIPv4() conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		pkt, ok := <-b.passThroughCh
		if !ok {
			return 0, net.ErrClosed
		}
		sizes[0] = copy(bufs[0], pkt.Data)
		eps[0] = &WRRPEndpoint{
			Addr:          pkt.Addr.AddrPort(),
			TransportType: ICE,
		}
		return 1, nil
	}
}

// makeReceiveIPv6 returns a ReceiveFunc that reads WireGuard packets from
// passThrough6Ch. FilteringUDPMux (v6) is the sole reader of v6conn and
// forwards non-STUN packets here, mirroring the v4 design and supporting
// ICE over IPv6 without a race with the mux's connWorker goroutine.
func (b *DefaultBind) makeReceiveIPv6() conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		pkt, ok := <-b.passThrough6Ch
		if !ok {
			return 0, net.ErrClosed
		}
		sizes[0] = copy(bufs[0], pkt.Data)
		eps[0] = &WRRPEndpoint{
			Addr:          pkt.Addr.AddrPort(),
			TransportType: ICE,
		}
		return 1, nil
	}
}

// TODO: When all Binds handle IdealBatchSize, remove this dynamic function and
// rename the IdealBatchSize constant to BatchSize.
func (b *DefaultBind) BatchSize() int {
	if runtime.GOOS == "linux" {
		return conn.IdealBatchSize
	}
	return 1
}

func (b *DefaultBind) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var err1, err2, err3 error
	if b.ipv4 != nil {
		err1 = b.ipv4.Close()
		b.ipv4 = nil
		b.ipv4PC = nil
	}
	if b.ipv6 != nil {
		err2 = b.ipv6.Close()
		b.ipv6 = nil
		b.ipv6PC = nil
	}

	b.blackhole4 = false
	b.blackhole6 = false
	if err1 != nil {
		return err1
	}

	if err2 != nil {
		return err2
	}
	return err3
}

func (b *DefaultBind) Send(bufs [][]byte, endpoint conn.Endpoint) error {
	// add drp write
	var e *WRRPEndpoint
	var ok bool
	if e, ok = endpoint.(*WRRPEndpoint); !ok {
		return fmt.Errorf("endpoint is not WRRPEndpoint")
	}

	if e.TransportType == WRRP {
		for _, buf := range bufs {
			err := b.wrrperClient.Send(context.Background(), e.RemoteId, wrrp.Forward, buf)
			if err != nil {
				return err
			}
		}
		return nil
	}

	b.mu.Lock()
	blackhole := b.blackhole4
	conn := b.ipv4
	var (
		pc4 *ipv4.PacketConn
		pc6 *ipv6.PacketConn
	)
	is6 := false
	if endpoint.DstIP().Is6() {
		blackhole = b.blackhole6
		conn = b.ipv6
		pc6 = b.ipv6PC
		is6 = true
	} else {
		pc4 = b.ipv4PC
	}
	b.mu.Unlock()

	if blackhole {
		return nil
	}
	if conn == nil {
		return syscall.EAFNOSUPPORT
	}

	if is6 {
		return b.send6(conn, pc6, endpoint, bufs)
	}

	return b.send4(b.v4conn, pc4, endpoint, bufs)
}

func (b *DefaultBind) send4(udpConn *net.UDPConn, pc *ipv4.PacketConn, ep conn.Endpoint, bufs [][]byte) error {
	ua := b.udpAddrPool.Get().(*net.UDPAddr)
	as4 := ep.DstIP().As4()
	copy(ua.IP, as4[:])
	ua.IP = ua.IP[:4]
	ua.Port = int(ep.(*WRRPEndpoint).Addr.Port())
	msgs := b.ipv4MsgsPool.Get().(*[]ipv4.Message)
	for i, buf := range bufs {
		(*msgs)[i].Buffers[0] = buf
		(*msgs)[i].Addr = ua
		setSrcControl(&(*msgs)[i].OOB, ep.(*WRRPEndpoint))
	}
	var (
		n     int
		err   error
		start int
	)
	if runtime.GOOS == "linux" {
		for {
			n, err = pc.WriteBatch((*msgs)[start:len(bufs)], 0)
			if err != nil || n == len((*msgs)[start:len(bufs)]) {
				break
			}
			start += n
		}
	} else {
		for i, buf := range bufs {
			_, _, err = udpConn.WriteMsgUDP(buf, (*msgs)[i].OOB, ua)
			if err != nil {
				break
			}
		}
	}
	b.udpAddrPool.Put(ua)
	b.ipv4MsgsPool.Put(msgs)
	return err
}

func (b *DefaultBind) send6(udpConn *net.UDPConn, pc *ipv6.PacketConn, ep conn.Endpoint, bufs [][]byte) error {
	ua := b.udpAddrPool.Get().(*net.UDPAddr)
	as16 := ep.DstIP().As16()
	copy(ua.IP, as16[:])
	//ua.IP = ua.IP[:16]
	//ua.Port = int(ep.(*internal.MagicEndpoint).Port())
	msgs := b.ipv6MsgsPool.Get().(*[]ipv6.Message)
	for i, buf := range bufs {
		(*msgs)[i].Buffers[0] = buf
		(*msgs)[i].Addr = ua
		setSrcControl(&(*msgs)[i].OOB, ep.(*WRRPEndpoint))
	}
	var (
		n     int
		err   error
		start int
	)
	if runtime.GOOS == "linux" {
		for {
			n, err = pc.WriteBatch((*msgs)[start:len(bufs)], 0)
			if err != nil || n == len((*msgs)[start:len(bufs)]) {
				break
			}
			start += n
		}
	} else {
		for i, buf := range bufs {
			_, _, err = udpConn.WriteMsgUDP(buf, (*msgs)[i].OOB, ua)
			if err != nil {
				break
			}
		}
	}
	b.udpAddrPool.Put(ua)
	b.ipv6MsgsPool.Put(msgs)
	return err
}
