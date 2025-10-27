package wrapper

import (
	"context"
	"errors"
	"github.com/wireflowio/ice"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	drpclient "wireflow/drp/client"
	"wireflow/internal"
	"wireflow/pkg/drp"
	"wireflow/pkg/log"
)

var (
	_ conn.Bind = (*LinkBind)(nil)
)

// LinkBind implements Bind for all platforms. While Windows has its own Bind
// (see bind_windows.go), it may fall back to LinkBind.
// TODO: RemoveProbe usage of ipv{4,6}.PacketConn when net.UDPConn has comparable
// methods for sending and receiving multiple datagrams per-syscall. See the
// proposal in https://github.com/golang/go/issues/45886#issuecomment-1218301564.
type LinkBind struct {
	logger          *log.Logger
	agent           *ice.Agent
	universalUdpMux *ice.UniversalUDPMuxDefault
	conn            net.Conn // drp client conn
	node            *drp.Node
	PublicKey       wgtypes.Key
	keyManager      internal.KeyManager

	// used for turn relay
	relayConn net.PacketConn

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
	V6Conn          *net.UDPConn
	UniversalUDPMux *ice.UniversalUDPMuxDefault
	RelayConn       net.PacketConn // relay conn, used for relay endpoint
	Proxy           *drpclient.Proxy
	KeyManager      internal.KeyManager
}

func NewBind(cfg *BindConfig) *LinkBind {
	return &LinkBind{
		logger:          cfg.Logger,
		proxy:           cfg.Proxy,
		v4conn:          cfg.V4Conn,
		v6conn:          cfg.V6Conn,
		universalUdpMux: cfg.UniversalUDPMux,
		keyManager:      cfg.KeyManager,
		relayConn:       cfg.RelayConn,
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

func (b *LinkBind) GetPackectConn4() net.PacketConn {
	return b.ipv4
}

func (b *LinkBind) GetPackectConn6() net.PacketConn {
	return b.ipv6
}

// ParseEndpoint there is throw kind of endpoint now, direct、drp and relay endpoint
// when it is direct, s can be parse to netip.AddrPort like 'xx.xx.xx.xx:port'
// when it is drp, s like 'drp:to=xxx//xx.xx.xx.xx:port'
// when it is relay, like 'relay://xx.xx.xx.xx:port'
func (b *LinkBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	if strings.HasPrefix(s, "drp:") {
		prefix, after, isExists := strings.Cut(s, "//")
		if !isExists {
			return nil, errors.New("invalid drp endpoint format, missing '//'")
		}

		e, err := netip.ParseAddrPort(after)
		if err != nil {
			return nil, err
		}

		// get To
		_, to, isExists := strings.Cut(prefix, "=")
		if !isExists {
			return nil, errors.New("invalid drp endpoint format, missing 'to='")
		}
		return &internal.LinkEndpoint{
			Relay: &struct {
				FromType internal.EndpointType
				Status   bool
				From     string
				To       string
				Endpoint netip.AddrPort
			}{FromType: internal.DRP, Status: true, From: b.keyManager.GetPublicKey(), To: to, Endpoint: e},
		}, nil
	} else if strings.HasPrefix(s, "relay:") {
		// relay endpoint
		var (
			isExists bool
			prefix   string
			after    string
		)
		pos := strings.LastIndex(s, "//")
		if pos == -1 {
			isExists = false
		} else {
			isExists = true
			prefix = s[:pos]
			after = s[pos+2:]
		}
		if !isExists {
			return nil, errors.New("invalid drp endpoint format, missing '//'")
		}

		e, err := netip.ParseAddrPort(after)
		if err != nil {
			return nil, err
		}
		_, to, isExists := strings.Cut(prefix, "=")
		if !isExists {
			return nil, errors.New("invalid drp endpoint format, missing 'to='")
		}
		return &internal.LinkEndpoint{
			Relay: &struct {
				FromType internal.EndpointType
				Status   bool
				From     string
				To       string
				Endpoint netip.AddrPort
			}{FromType: internal.Relay, Status: true, From: b.keyManager.GetPublicKey(), To: to, Endpoint: e},
		}, nil
	}
	e, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}

	return &internal.LinkEndpoint{
		Direct: struct{ AddrPort netip.AddrPort }{AddrPort: e},
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
func (b *LinkBind) Open(uport uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.ipv4 != nil || b.ipv6 != nil {
		return nil, 0, conn.ErrBindAlreadyOpen
	}

	port := int(uport)
	var v4pc *ipv4.PacketConn
	var v6pc *ipv6.PacketConn

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

	if b.proxy != nil {
		go func() {
			for {
				if err := b.proxy.Start(); err != nil {
					b.logger.Errorf("handle drp message error: %v, after 1s retry", err)
					time.Sleep(1 * time.Second)
				}
			}
		}()
		fns = append(fns, b.makeReceiveDrp())
	}

	if b.relayConn != nil {
		fns = append(fns, b.makeReceiveRelay())
	}

	return fns, uint16(port), nil
}

// makeReceiveDrp will receive data from drp server, using grpc transport.
// It will return a conn.ReceiveFunc that can be used to receive data from the drp server.
func (b *LinkBind) makeReceiveDrp() conn.ReceiveFunc {
	return b.proxy.MakeReceiveFromDrp()
}

func (b *LinkBind) makeReceiveRelay() conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		n, addr, err := b.relayConn.ReadFrom(bufs[0])
		if err != nil {
			return 0, err
		}
		//	MessageInitiationType  = 1
		//	MessageResponseType    = 2
		//	MessageCookieReplyType = 3
		//	MessageTransportType   = 4
		//msgType := binary.LittleEndian.Uint32(bufs[0][:4])
		//b.logger.Verbosef("receive msgType: %v, addr: %v, data: %v", msgType, addr, bufs[0][:n])
		sizes[0] = n
		addrPort, err := netip.ParseAddrPort(addr.String())
		if err != nil {
			b.logger.Errorf("err: %v", err)
			return 0, err
		}

		eps[0] = &internal.LinkEndpoint{
			Relay: &struct {
				FromType internal.EndpointType
				Status   bool
				From     string
				To       string
				Endpoint netip.AddrPort
			}{FromType: internal.Relay, Status: true, From: "", To: b.keyManager.GetPublicKey(), Endpoint: addrPort},
		}

		return 1, nil
	}
}

func (b *LinkBind) makeReceiveIPv4(pc *ipv4.PacketConn, udpConn *net.UDPConn) conn.ReceiveFunc {
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
			ep := &internal.LinkEndpoint{
				Direct: struct{ AddrPort netip.AddrPort }{AddrPort: addrPort},
			} // TODO: remove allocation
			getSrcFromControl(msg.OOB[:msg.NN], ep)
			eps[i] = ep
		}
		return numMsgs, nil
	}
}

func (b *LinkBind) makeReceiveIPv6(pc *ipv6.PacketConn, udpConn *net.UDPConn) conn.ReceiveFunc {
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
				b.logger.Errorf("handle stun message error: %v", err)
				return 0, nil
			}

			if ok {
				return 0, nil
			}
			sizes[i] = msg.N
			addrPort := msg.Addr.(*net.UDPAddr).AddrPort()
			ep := &internal.LinkEndpoint{Direct: struct{ AddrPort netip.AddrPort }{AddrPort: addrPort}} // TODO: remove allocation
			getSrcFromControl(msg.OOB[:msg.NN], ep)
			eps[i] = ep
		}
		return numMsgs, nil
	}
}

// TODO: When all Binds handle IdealBatchSize, remove this dynamic function and
// rename the IdealBatchSize constant to BatchSize.
func (b *LinkBind) BatchSize() int {
	if runtime.GOOS == "linux" {
		return conn.IdealBatchSize
	}
	return 1
}

func (b *LinkBind) Close() error {
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

func (b *LinkBind) Send(bufs [][]byte, endpoint conn.Endpoint) error {
	// add drp write
	if v, ok := endpoint.(internal.RelayEndpoint); ok {
		switch v.FromType() {
		case internal.DRP:
			if b.proxy == nil {
				return errors.New("proxy is nil, please set proxy first")
			}
			if err := b.proxy.Send(endpoint, bufs); err != nil {
				return err
			}
			return nil
		case internal.Relay:
			if b.relayConn == nil {
				return errors.New("relayConn is nil, please set relayConn first")
			}

			addr, err := net.ResolveUDPAddr("udp", endpoint.DstToString())
			if err != nil {
				return err
			}

			for _, buf := range bufs {
				b.logger.Verbosef("send data to: %v, data: %v", addr, buf)
				if _, err := b.relayConn.WriteTo(buf, addr); err != nil {
					b.logger.Errorf("send relay message error: %v", err)
					return err
				}
			}

			return nil
		}

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

func (b *LinkBind) send4(conn *net.UDPConn, pc *ipv4.PacketConn, ep conn.Endpoint, bufs [][]byte) error {
	ua := b.udpAddrPool.Get().(*net.UDPAddr)
	as4 := ep.DstIP().As4()
	copy(ua.IP, as4[:])
	ua.IP = ua.IP[:4]
	ua.Port = int(ep.(*internal.LinkEndpoint).Direct.AddrPort.Port())
	msgs := b.ipv4MsgsPool.Get().(*[]ipv4.Message)
	for i, buf := range bufs {
		(*msgs)[i].Buffers[0] = buf
		(*msgs)[i].Addr = ua
		setSrcControl(&(*msgs)[i].OOB, ep.(*internal.LinkEndpoint))
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

func (b *LinkBind) send6(conn *net.UDPConn, pc *ipv6.PacketConn, ep conn.Endpoint, bufs [][]byte) error {
	ua := b.udpAddrPool.Get().(*net.UDPAddr)
	as16 := ep.DstIP().As16()
	copy(ua.IP, as16[:])
	//ua.IP = ua.IP[:16]
	//ua.Port = int(ep.(*internal.LinkEndpoint).Port())
	msgs := b.ipv6MsgsPool.Get().(*[]ipv6.Message)
	for i, buf := range bufs {
		(*msgs)[i].Buffers[0] = buf
		(*msgs)[i].Addr = ua
		setSrcControl(&(*msgs)[i].OOB, ep.(*internal.LinkEndpoint))
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
