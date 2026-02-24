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

	"github.com/wireflowio/ice"
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
	logger          *log.Logger
	universalUdpMux *ice.UniversalUDPMuxDefault
	PublicKey       wgtypes.Key
	keyManager      KeyManager

	// used for turn relay
	wrrperClient Wrrp

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
	Logger          *log.Logger
	V4Conn          *net.UDPConn
	V6Conn          *net.UDPConn
	UniversalUDPMux *ice.UniversalUDPMuxDefault
	WrrpClient      Wrrp
	KeyManager      KeyManager
}

func NewBind(cfg *BindConfig) *DefaultBind {
	return &DefaultBind{
		logger:          cfg.Logger,
		v4conn:          cfg.V4Conn,
		v6conn:          cfg.V6Conn,
		universalUdpMux: cfg.UniversalUDPMux,
		keyManager:      cfg.KeyManager,
		wrrperClient:    cfg.WrrpClient,
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
		_, after, b := strings.Cut(s, "wrrp://")
		if !b {
			return nil, errors.New("invalid endpoint")
		}
		remoteId, err := strconv.ParseUint(after, 10, 64)
		if err != nil {
			return nil, err
		}
		e, err := netip.ParseAddrPort(config.Conf.WrrperURL)
		if err != nil {
			return nil, err
		}
		return &WRRPEndpoint{
			Addr:          e,
			RemoteId:      remoteId,
			TransportType: WRRP,
		}, nil
	}
	e, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
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

// Open copy from wiregaurd, add a drp ReceiveFunc
func (b *DefaultBind) Open(uport uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.ipv4 != nil || b.ipv6 != nil {
		return nil, 0, conn.ErrBindAlreadyOpen
	}

	port := int(uport)
	var v4pc *ipv4.PacketConn
	var v6pc *ipv6.PacketConn

	// ReceiveFunc on the same port as we're using for ipv4.
	var fns []conn.ReceiveFunc
	if b.v4conn != nil {
		if runtime.GOOS == "linux" {
			v4pc = ipv4.NewPacketConn(b.v4conn)
			b.ipv4PC = v4pc
		}
		fns = append(fns, b.makeReceiveIPv4(v4pc, b.v4conn))
		b.ipv4 = b.v4conn
	}

	if b.v6conn != nil {
		if runtime.GOOS == "linux" {
			v6pc = ipv6.NewPacketConn(b.v6conn)
			b.ipv6PC = v6pc
		}
		fns = append(fns, b.makeReceiveIPv6(v6pc, b.v6conn))
		b.ipv6 = b.v6conn
	}
	if len(fns) == 0 {
		return nil, 0, syscall.EAFNOSUPPORT
	}

	// add wireflow wrrp
	if config.Conf.EnableWrrp {
		fns = append(fns, b.wrrperClient.ReceiveFunc())
	}

	return fns, uint16(port), nil
}

func (b *DefaultBind) makeReceiveIPv4(pc *ipv4.PacketConn, udpConn *net.UDPConn) conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		msgs := b.ipv4MsgsPool.Get().(*[]ipv4.Message)
		defer b.ipv4MsgsPool.Put(msgs)
		for i := range bufs {
			(*msgs)[i].Buffers[0] = bufs[i]
		}

		var numMsgs int
		if runtime.GOOS == "linux" {
			numMsgs, err = pc.ReadBatch(*msgs, 0)
			if err != nil {
				return 0, err
			}
		} else {
			msg := &(*msgs)[0]
			msg.N, msg.NN, _, msg.Addr, err = udpConn.ReadMsgUDP(msg.Buffers[0], msg.OOB)
			if err != nil {
				return 0, err
			}
			numMsgs = 1
		}
		for i := 0; i < numMsgs; i++ {
			msg := &(*msgs)[i]
			//here should hand stun message

			ok, err := b.universalUdpMux.FilterMessage(msg.Buffers[0], msg.N, msg.Addr.(*net.UDPAddr))
			if err != nil {
				b.logger.Error("handle stun message error", err)
				return 0, nil
			}

			if ok {
				return 0, nil
			}

			sizes[i] = msg.N
			addrPort := msg.Addr.(*net.UDPAddr).AddrPort()
			ep := &WRRPEndpoint{
				Addr:          addrPort,
				TransportType: ICE,
			} // TODO: remove allocation
			getSrcFromControl(msg.OOB[:msg.NN], ep)
			eps[i] = ep
		}
		return numMsgs, nil
	}
}

func (b *DefaultBind) makeReceiveIPv6(pc *ipv6.PacketConn, udpConn *net.UDPConn) conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		msgs := b.ipv6MsgsPool.Get().(*[]ipv6.Message)
		defer b.ipv6MsgsPool.Put(msgs)
		for i := range bufs {
			(*msgs)[i].Buffers[0] = bufs[i]
		}
		var numMsgs int
		if runtime.GOOS == "linux" {
			numMsgs, err = pc.ReadBatch(*msgs, 0)
			if err != nil {
				return 0, err
			}
		} else {
			msg := &(*msgs)[0]
			msg.N, msg.NN, _, msg.Addr, err = udpConn.ReadMsgUDP(msg.Buffers[0], msg.OOB)
			if err != nil {
				return 0, err
			}
			numMsgs = 1
		}
		for i := 0; i < numMsgs; i++ {
			msg := &(*msgs)[i]

			ok, err := b.universalUdpMux.FilterMessage(msg.Buffers[0], msg.N, msg.Addr.(*net.UDPAddr))
			if err != nil {
				b.logger.Error("handle stun message error", err)
				return 0, nil
			}

			if ok {
				return 0, nil
			}
			sizes[i] = msg.N
			addrPort := msg.Addr.(*net.UDPAddr).AddrPort()
			ep := &WRRPEndpoint{
				Addr:          addrPort,
				TransportType: ICE,
			} // TODO: remove allocation
			getSrcFromControl(msg.OOB[:msg.NN], ep)
			eps[i] = ep
		}
		return numMsgs, nil
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
