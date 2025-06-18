package wrapper

import (
	"context"
	"errors"
	"github.com/linkanyio/ice"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	drpclient "linkany/drp/client"
	"linkany/internal"
	"linkany/pkg/drp"
	"linkany/pkg/log"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var (
	_ conn.Bind = (*NetBind)(nil)
)

// NetBind implements Bind for all platforms. While Windows has its own Bind
// (see bind_windows.go), it may fall back to NetBind.
// TODO: Remove usage of ipv{4,6}.PacketConn when net.UDPConn has comparable
// methods for sending and receiving multiple datagrams per-syscall. See the
// proposal in https://github.com/golang/go/issues/45886#issuecomment-1218301564.
type NetBind struct {
	logger          *log.Logger
	agent           *ice.Agent
	universalUdpMux *ice.UniversalUDPMuxDefault
	conn            net.Conn // drp client conn
	node            *drp.Node
	Publikey        wgtypes.Key
	drpClient       *drpclient.Client                // drpclient using grpc to connect to drp server receiving data.
	dstConns        map[conn.Endpoint]net.PacketConn // destination conn

	proxy *drpclient.Proxy

	drpAddr net.TCPAddr // drp addr，drp created from console

	mu     sync.Mutex // protects all fields except as specified
	v4conn *net.UDPConn
	v6conn *net.UDPConn
	port   int
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
	UniversalUDPMux *ice.UniversalUDPMuxDefault
	DrpClient       *drpclient.Client
	Proxy           *drpclient.Proxy
}

func NewBind(cfg *BindConfig) *NetBind {
	return &NetBind{
		logger:          cfg.Logger,
		drpClient:       cfg.DrpClient,
		proxy:           cfg.Proxy,
		dstConns:        make(map[conn.Endpoint]net.PacketConn),
		v4conn:          cfg.V4Conn,
		universalUdpMux: cfg.UniversalUDPMux,
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

func (b *NetBind) GetPackectConn4() net.PacketConn {
	return b.ipv4
}

func (b *NetBind) GetPackectConn6() net.PacketConn {
	return b.ipv6
}

// ParseEndpoint when the endpoint is relay, will add a flag 'true' to anyEndpoint
func (b *NetBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	e, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return &internal.RemoteEndpoint{
		AddrPort: e,
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
	v4conn, port, err := listenNet("udp4", port)
	if err != nil && !errors.Is(err, syscall.EAFNOSUPPORT) {
		return nil, 0, err
	}

	return v4conn, port, nil
}

// Open copy from wiregaurd, add a drp ReceiveFunc
func (b *NetBind) Open(uport uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.ipv4 != nil || b.ipv6 != nil {
		return nil, 0, conn.ErrBindAlreadyOpen
	}

	port := int(uport)
	var v4pc *ipv4.PacketConn

	// Listen on the same port as we're using for ipv4.
	var fns []conn.ReceiveFunc
	if b.v4conn != nil {
		if runtime.GOOS == "linux" {
			v4pc = ipv4.NewPacketConn(b.v4conn)
			b.ipv4PC = v4pc
		}
		fns = append(fns, b.makeReceiveIPv4(v4pc, b.v4conn))
		b.ipv4 = b.v4conn
	}
	if len(fns) == 0 {
		return nil, 0, syscall.EAFNOSUPPORT
	}

	if b.drpClient != nil {
		go func() {
			for {
				if err := b.drpClient.HandleMessage(context.Background(), b.proxy); err != nil {
					b.logger.Errorf("handle drp message error: %v, after 1s retry", err)
					time.Sleep(1 * time.Second)
				}
			}
		}()
		fns = append(fns, b.makeReceiveDrp())
	}

	return fns, uint16(port), nil
}

// makeReceiveDrp will receive data from drp server, using grpc transport.
// It will return a conn.ReceiveFunc that can be used to receive data from the drp server.
func (b *NetBind) makeReceiveDrp() conn.ReceiveFunc {
	return b.proxy.Receive()
}

func (b *NetBind) makeReceiveIPv4(pc *ipv4.PacketConn, udpConn *net.UDPConn) conn.ReceiveFunc {
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
				b.logger.Errorf("handle stun message error: %v", err)
				return 0, nil
			}

			if ok {
				return 0, nil
			}

			sizes[i] = msg.N
			addrPort := msg.Addr.(*net.UDPAddr).AddrPort()
			ep := &internal.RemoteEndpoint{AddrPort: addrPort} // TODO: remove allocation
			getSrcFromControl(msg.OOB[:msg.NN], ep)
			eps[i] = ep
		}
		return numMsgs, nil
	}
}

func (b *NetBind) makeReceiveIPv6(pc *ipv6.PacketConn, udpConn *net.UDPConn) conn.ReceiveFunc {
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
			sizes[i] = msg.N
			addrPort := msg.Addr.(*net.UDPAddr).AddrPort()
			ep := &internal.RemoteEndpoint{AddrPort: addrPort} // TODO: remove allocation
			getSrcFromControl(msg.OOB[:msg.NN], ep)
			eps[i] = ep
		}
		return numMsgs, nil
	}
}

// TODO: When all Binds handle IdealBatchSize, remove this dynamic function and
// rename the IdealBatchSize constant to BatchSize.
func (b *NetBind) BatchSize() int {
	if runtime.GOOS == "linux" {
		return conn.IdealBatchSize
	}
	return 1
}

func (b *NetBind) Close() error {
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

func (b *NetBind) Send(bufs [][]byte, endpoint conn.Endpoint) error {
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

	// add drp write
	if ep := b.dstConns[endpoint]; ep != nil {
		addr, err := net.ResolveUDPAddr("udp", endpoint.DstToString())
		if err != nil {
			return err
		}
		_, err = ep.WriteTo(bufs[0], addr)
		if err != nil {
			return err
		}
	}
	if is6 {
		return b.send6(conn, pc6, endpoint, bufs)
	}

	return b.send4(b.v4conn, pc4, endpoint, bufs)
}

func (b *NetBind) send4(conn *net.UDPConn, pc *ipv4.PacketConn, ep conn.Endpoint, bufs [][]byte) error {
	ua := b.udpAddrPool.Get().(*net.UDPAddr)
	as4 := ep.DstIP().As4()
	copy(ua.IP, as4[:])
	ua.IP = ua.IP[:4]
	ua.Port = int(ep.(*internal.RemoteEndpoint).Port())
	msgs := b.ipv4MsgsPool.Get().(*[]ipv4.Message)
	for i, buf := range bufs {
		(*msgs)[i].Buffers[0] = buf
		(*msgs)[i].Addr = ua
		setSrcControl(&(*msgs)[i].OOB, ep.(*internal.RemoteEndpoint))
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
			_, _, err = conn.WriteMsgUDP(buf, (*msgs)[i].OOB, ua)
			if err != nil {
				break
			}
		}
	}
	b.udpAddrPool.Put(ua)
	b.ipv4MsgsPool.Put(msgs)
	return err
}

func (b *NetBind) send6(conn *net.UDPConn, pc *ipv6.PacketConn, ep conn.Endpoint, bufs [][]byte) error {
	ua := b.udpAddrPool.Get().(*net.UDPAddr)
	as16 := ep.DstIP().As16()
	copy(ua.IP, as16[:])
	ua.IP = ua.IP[:16]
	ua.Port = int(ep.(*internal.RemoteEndpoint).Port())
	msgs := b.ipv6MsgsPool.Get().(*[]ipv6.Message)
	for i, buf := range bufs {
		(*msgs)[i].Buffers[0] = buf
		(*msgs)[i].Addr = ua
		setSrcControl(&(*msgs)[i].OOB, ep.(*internal.RemoteEndpoint))
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
			_, _, err = conn.WriteMsgUDP(buf, (*msgs)[i].OOB, ua)
			if err != nil {
				break
			}
		}
	}
	b.udpAddrPool.Put(ua)
	b.ipv6MsgsPool.Put(msgs)
	return err
}

func (b *NetBind) SetEndpoint(addr net.Addr, conn net.PacketConn) error {
	endpoint, err := b.ParseEndpoint(addr.String())
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dstConns[endpoint] = conn
	return nil
}
