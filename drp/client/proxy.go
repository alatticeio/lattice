package client

import (
	"context"
	"errors"
	"golang.zx2c4.com/wireguard/conn"
	"net"
	"net/netip"
	"time"
	drpgrpc "wireflow/drp/grpc"
	"wireflow/internal"
	"wireflow/pkg/log"
)

// Proxy will send data to local engine when using drp relay,
// and also send data to other node from local engine
type Proxy struct {
	logger *log.Logger
	// Address is the address of the proxy server
	Addr          netip.AddrPort
	outBoundQueue chan *drpgrpc.DrpMessage
	inBoundQueue  chan *drpgrpc.DrpMessage
	drpAddr       string
	drpClient     *Client
	offerHandler  internal.OfferHandler
	msgManager    *MessageManager
	probeManager  internal.ProbeManager

	proxyDo func(ctx context.Context, msg *drpgrpc.DrpMessage) error
}

type ProxyConfig struct {
	OfferHandler internal.OfferHandler
	DrpClient    *Client
	DrpAddr      string
}

func NewProxy(cfg *ProxyConfig) (*Proxy, error) {
	addr, err := net.ResolveTCPAddr("tcp", cfg.DrpAddr)
	if err != nil {
		return nil, err
	}
	addrPort := addr.AddrPort()
	return &Proxy{
		outBoundQueue: make(chan *drpgrpc.DrpMessage, 10000),
		inBoundQueue:  make(chan *drpgrpc.DrpMessage, 10000), // Buffered channel to handle messages
		logger:        log.NewLogger(log.Loglevel, "proxy"),
		offerHandler:  cfg.OfferHandler,
		drpClient:     cfg.DrpClient,
		msgManager:    NewMessageManager(),
		Addr:          addrPort, // Default address, can be set later
	}, nil
}

func (p *Proxy) OfferAndProbe(offerHandler internal.OfferHandler, probeManager internal.ProbeManager) *Proxy {
	p.offerHandler = offerHandler
	p.probeManager = probeManager
	return p
}

func (p *Proxy) Start() error {
	return p.drpClient.HandleMessage(context.Background(), p.outBoundQueue, p.ReceiveMessage)
}

// ReceiveMessage receive message from drp server
func (p *Proxy) ReceiveMessage(ctx context.Context, msg *drpgrpc.DrpMessage) error {
	if msg.Body == nil {
		return errors.New("body is nil")
	}

	switch msg.MsgType {
	case drpgrpc.MessageType_MessageDrpDataType:
		// write data
		p.inBoundQueue <- msg
	default:
		p.logger.Verbosef("receive from drp server type: %v, from: %v, to: %v", msg.MsgType, msg.From, msg.To)

		go func() {
			if err := p.offerHandler.ReceiveOffer(ctx, msg); err != nil {
				p.logger.Errorf("handle response failed: %v", err)
			}
		}()
	}

	return nil
}

func (p *Proxy) MakeReceiveFromDrp() conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		msg := <-p.inBoundQueue
		// write to local engine
		switch msg.MsgType {
		case drpgrpc.MessageType_MessageDrpDataType:
			p.logger.Verbosef("client received drp data, time slapped: %v", time.Since(time.UnixMilli(msg.Timestamp)).Milliseconds())
			for i := 0; i < len(bufs); i++ {
				copy(bufs[i], msg.Body)
				sizes[i] = len(msg.Body)
				eps[i] = &internal.LinkEndpoint{
					Relay: &struct {
						FromType internal.EndpointType
						Status   bool
						From     string
						To       string
						Endpoint netip.AddrPort
					}{FromType: internal.DRP, Status: true, From: msg.To, To: msg.From, Endpoint: p.Addr},
				}
			}

			return len(bufs), nil
		default:
			p.logger.Errorf("unsupported message type: %v", msg.MsgType)
			return -1, errors.New("unsupported message type")
		}
	}

}

func (p *Proxy) Send(ep conn.Endpoint, bufs [][]byte) (err error) {
	var (
		from string
		to   string
	)
	if ep == nil {
		return errors.New("endpoint is nil")
	}

	if v, ok := ep.(internal.RelayEndpoint); ok {
		from = v.From()
		to = v.To()
	} else {
		return errors.New("unsupported endpoint type")
	}

	for i := 0; i < len(bufs); i++ {
		if bufs[i] == nil || len(bufs[i]) == 0 {
			return errors.New("buffer is nil or empty")
		}
		drpMesssage := p.GetMessageFromPool()
		drpMesssage.From = from
		drpMesssage.To = to
		drpMesssage.MsgType = drpgrpc.MessageType_MessageDrpDataType
		drpMesssage.Body = bufs[i]
		drpMesssage.Timestamp = time.Now().UnixMilli()

		p.outBoundQueue <- drpMesssage
	}

	return nil
}

// WriteMessage will send actual message to data channel
func (p *Proxy) WriteMessage(ctx context.Context, msg *drpgrpc.DrpMessage) error {
	p.outBoundQueue <- msg
	return nil
}
